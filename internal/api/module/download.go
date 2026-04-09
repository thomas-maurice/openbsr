package module

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DownloadService struct {
	modulev1connect.UnimplementedDownloadServiceHandler
	repos *iface.Repos
	store storage.Store
}

func NewDownloadService(repos *iface.Repos, store storage.Store) *DownloadService {
	return &DownloadService{repos: repos, store: store}
}

func (s *DownloadService) Download(
	ctx context.Context,
	req *connect.Request[modulev1.DownloadRequest],
) (*connect.Response[modulev1.DownloadResponse], error) {
	caller := auth.UserFromContext(ctx)

	var contents []*modulev1.DownloadResponse_Content
	for _, v := range req.Msg.GetValues() {
		m, commit, err := resolveResourceRef(ctx, s.repos, v.GetResourceRef())
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if commit == nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no commits found for module"))
		}

		// Access check
		if !s.repos.Auth.CanReadModule(ctx, userID(caller), m) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
		}

		// Retrieve blob from storage
		data, err := s.store.Get(ctx, commit.StorageKey)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to retrieve content"))
		}

		// Unzip and return files
		files, err := unzipFiles(data)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to read content"))
		}

		ownerID := resolveOwnerIDHelper(ctx, s.repos, m)
		contents = append(contents, &modulev1.DownloadResponse_Content{
			Commit: &modulev1.Commit{
				Id:              commit.ID,
				CreateTime:      timestamppb.New(commit.CreatedAt),
				OwnerId:         ownerID,
				ModuleId:        m.ID,
				CreatedByUserId: commit.CreatedByUserID,
			},
			Files: files,
		})
	}

	return connect.NewResponse(&modulev1.DownloadResponse{Contents: contents}), nil
}
