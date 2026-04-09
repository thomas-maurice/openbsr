package module

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"

	"connectrpc.com/connect"
	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"github.com/google/uuid"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
	"github.com/thomas-maurice/openbsr/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	maxUploadSize = 50 * 1024 * 1024 // 50 MB total
	maxFileCount  = 10000
	maxFileSize   = 10 * 1024 * 1024 // 10 MB per file
)

type UploadService struct {
	modulev1connect.UnimplementedUploadServiceHandler
	repos *iface.Repos
	store storage.Store
}

func NewUploadService(repos *iface.Repos, store storage.Store) *UploadService {
	return &UploadService{repos: repos, store: store}
}

func (s *UploadService) Upload(
	ctx context.Context,
	req *connect.Request[modulev1.UploadRequest],
) (*connect.Response[modulev1.UploadResponse], error) {
	caller := auth.UserFromContext(ctx)
	if caller == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	var commits []*modulev1.Commit
	for _, content := range req.Msg.GetContents() {
		modRef := content.GetModuleRef()
		if modRef == nil || modRef.GetName() == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("module ref with name required"))
		}
		ownerName := modRef.GetName().GetOwner()
		moduleName := modRef.GetName().GetModule()

		// Look up module
		m, err := s.repos.Modules.Get(ctx, ownerName, moduleName)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found: "+ownerName+"/"+moduleName))
		}

		// Write access check: must be owner or org member (regardless of visibility)
		if !s.repos.Auth.CanWriteModule(ctx, caller.ID, m) {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("no write access"))
		}

		// Validate file paths and enforce size limits
		if err := validateFiles(content.GetFiles()); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Zip the files
		zipData, err := zipFiles(content.GetFiles())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create archive"))
		}
		if len(zipData) > maxUploadSize {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("upload too large"))
		}

		// Commit ID = SHA-256 of zip content (content-addressable)
		hash := sha256.Sum256(zipData)
		commitID := hex.EncodeToString(hash[:])
		digestValue := hash[:]
		storageKey := m.ID + "/" + commitID

		// Store blob
		if err := s.store.Put(ctx, storageKey, zipData); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to store content"))
		}

		// Create commit (skip if same content was already pushed)
		now := time.Now().UTC()
		existing, _ := s.repos.Commits.GetByID(ctx, m.ID, commitID)
		if existing != nil {
			// Same content already pushed — reuse existing commit
			now = existing.CreatedAt
		} else {
			c := &model.Commit{
				ID:              commitID,
				ModuleID:        m.ID,
				StorageKey:      storageKey,
				CreatedByUserID: caller.ID,
				CreatedAt:       now,
			}
			if err := s.repos.Commits.Create(ctx, c); err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create commit"))
			}
		}

		// Upsert labels (default to "main" if none specified)
		labelNames := []string{"main"}
		if len(content.GetScopedLabelRefs()) > 0 {
			labelNames = nil
			for _, ref := range content.GetScopedLabelRefs() {
				name := ref.GetName()
				if name == "" {
					name = "main"
				}
				labelNames = append(labelNames, name)
			}
		}
		for _, labelName := range labelNames {
			l := &model.Label{
				ID:        uuid.NewString(),
				ModuleID:  m.ID,
				Name:      labelName,
				CommitID:  commitID,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := s.repos.Labels.Upsert(ctx, l); err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("failed to upsert label"))
			}
		}

		// Build proto commit
		ownerID := resolveOwnerIDHelper(ctx, s.repos, m)
		commits = append(commits, &modulev1.Commit{
			Id:              commitID,
			CreateTime:      timestamppb.New(now),
			OwnerId:         ownerID,
			ModuleId:        m.ID,
			CreatedByUserId: caller.ID,
			Digest: &modulev1.Digest{
				Type:  modulev1.DigestType_DIGEST_TYPE_B5,
				Value: digestValue,
			},
		})
	}

	return connect.NewResponse(&modulev1.UploadResponse{Commits: commits}), nil
}

// validateFiles checks file paths for safety and enforces size limits.
func validateFiles(files []*modulev1.File) error {
	if len(files) > maxFileCount {
		return errors.New("too many files")
	}
	totalSize := 0
	for _, f := range files {
		p := f.GetPath()
		if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") ||
			strings.Contains(p, "..") {
			return errors.New("invalid file path: " + p)
		}
		if len(f.GetContent()) > maxFileSize {
			return errors.New("file too large: " + p)
		}
		totalSize += len(f.GetContent())
		if totalSize > maxUploadSize {
			return errors.New("total upload too large")
		}
	}
	return nil
}

func zipFiles(files []*modulev1.File) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.Create(f.GetPath())
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(f.GetContent()); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func unzipFiles(data []byte) ([]*modulev1.File, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var files []*modulev1.File
	var totalSize int64
	for _, f := range r.File {
		if f.UncompressedSize64 > maxFileSize {
			return nil, errors.New("decompressed file too large: " + f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		limited := io.LimitReader(rc, maxFileSize+1)
		var content bytes.Buffer
		n, err := content.ReadFrom(limited)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if n > maxFileSize {
			return nil, errors.New("decompressed file too large: " + f.Name)
		}
		totalSize += n
		if totalSize > maxUploadSize {
			return nil, errors.New("total decompressed size too large")
		}
		files = append(files, &modulev1.File{
			Path:    f.Name,
			Content: content.Bytes(),
		})
	}
	return files, nil
}
