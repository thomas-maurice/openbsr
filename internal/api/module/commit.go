package module

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CommitService struct {
	modulev1connect.UnimplementedCommitServiceHandler
	repos *iface.Repos
}

func NewCommitService(repos *iface.Repos) *CommitService {
	return &CommitService{repos: repos}
}

func (s *CommitService) GetCommits(
	ctx context.Context,
	req *connect.Request[modulev1.GetCommitsRequest],
) (*connect.Response[modulev1.GetCommitsResponse], error) {
	caller := auth.UserFromContext(ctx)
	var commits []*modulev1.Commit
	for _, ref := range req.Msg.GetResourceRefs() {
		m, commit, err := resolveResourceRef(ctx, s.repos, ref)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if !s.repos.Auth.CanReadModule(ctx, userID(caller), m) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
		}
		if commit == nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no commits found"))
		}
		ownerID := resolveOwnerIDHelper(ctx, s.repos, m)
		commits = append(commits, commitToProto(commit, ownerID))
	}
	return connect.NewResponse(&modulev1.GetCommitsResponse{Commits: commits}), nil
}

func (s *CommitService) ListCommits(
	ctx context.Context,
	req *connect.Request[modulev1.ListCommitsRequest],
) (*connect.Response[modulev1.ListCommitsResponse], error) {
	caller := auth.UserFromContext(ctx)
	ref := req.Msg.GetResourceRef()
	m, _, err := resolveResourceRef(ctx, s.repos, ref)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if !s.repos.Auth.CanReadModule(ctx, userID(caller), m) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
	}

	dbCommits, err := s.repos.Commits.ListByModule(ctx, m.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	ownerID := resolveOwnerIDHelper(ctx, s.repos, m)
	var commits []*modulev1.Commit
	for _, c := range dbCommits {
		commits = append(commits, commitToProto(c, ownerID))
	}
	return connect.NewResponse(&modulev1.ListCommitsResponse{Commits: commits}), nil
}

func commitToProto(c *model.Commit, ownerID string) *modulev1.Commit {
	return &modulev1.Commit{
		Id:              c.ID,
		CreateTime:      timestamppb.New(c.CreatedAt),
		OwnerId:         ownerID,
		ModuleId:        c.ModuleID,
		CreatedByUserId: c.CreatedByUserID,
	}
}
