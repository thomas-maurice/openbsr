package module

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/storage"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FileDescriptorSetService struct {
	modulev1connect.UnimplementedFileDescriptorSetServiceHandler
	repos *iface.Repos
	store storage.Store
}

func NewFileDescriptorSetService(repos *iface.Repos, store storage.Store) *FileDescriptorSetService {
	return &FileDescriptorSetService{repos: repos, store: store}
}

func (s *FileDescriptorSetService) GetFileDescriptorSet(
	ctx context.Context,
	req *connect.Request[modulev1.GetFileDescriptorSetRequest],
) (*connect.Response[modulev1.GetFileDescriptorSetResponse], error) {
	caller := auth.UserFromContext(ctx)

	m, commit, err := resolveResourceRef(ctx, s.repos, req.Msg.GetResourceRef())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if !s.repos.Auth.CanReadModule(ctx, userID(caller), m) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
	}
	if commit == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no commits found"))
	}

	// Get stored files
	data, err := s.store.Get(ctx, commit.StorageKey)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to retrieve content"))
	}
	files, err := unzipFiles(data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to read content"))
	}

	// Build an in-memory filesystem for protocompile
	fileMap := make(map[string]string)
	var protoFiles []string
	for _, f := range files {
		fileMap[f.GetPath()] = string(f.GetContent())
		if strings.HasSuffix(f.GetPath(), ".proto") {
			protoFiles = append(protoFiles, f.GetPath())
		}
	}

	if len(protoFiles) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no .proto files found"))
	}
	if len(protoFiles) > 500 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("too many proto files"))
	}

	// Compile with a hard 30-second timeout
	compileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	compiler := protocompile.Compiler{
		Resolver: &protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(fileMap),
		},
		Reporter: reporter.NewReporter(
			func(err reporter.ErrorWithPos) error { return err },
			nil,
		),
	}

	compiled, err := compiler.Compile(compileCtx, protoFiles...)
	if err != nil {
		slog.Error("proto compilation failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("proto compilation failed"))
	}

	// Build FileDescriptorSet from compiled results
	fds := &descriptorpb.FileDescriptorSet{}
	for _, result := range compiled {
		fdp := protodesc.ToFileDescriptorProto(result)
		fds.File = append(fds.File, fdp)
	}

	ownerID := resolveOwnerIDHelper(ctx, s.repos, m)
	resp := &modulev1.GetFileDescriptorSetResponse{
		FileDescriptorSet: fds,
		Commit: &modulev1.Commit{
			Id:              commit.ID,
			CreateTime:      timestamppb.New(commit.CreatedAt),
			OwnerId:         ownerID,
			ModuleId:        m.ID,
			CreatedByUserId: commit.CreatedByUserID,
		},
	}
	return connect.NewResponse(resp), nil
}
