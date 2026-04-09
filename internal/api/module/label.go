package module

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"github.com/google/uuid"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type LabelService struct {
	modulev1connect.UnimplementedLabelServiceHandler
	repos *iface.Repos
}

func NewLabelService(repos *iface.Repos) *LabelService {
	return &LabelService{repos: repos}
}

func (s *LabelService) GetLabels(
	ctx context.Context,
	req *connect.Request[modulev1.GetLabelsRequest],
) (*connect.Response[modulev1.GetLabelsResponse], error) {
	caller := auth.UserFromContext(ctx)
	var labels []*modulev1.Label
	for _, ref := range req.Msg.GetLabelRefs() {
		if ref.GetName() == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("label ref must have name"))
		}
		n := ref.GetName()
		m, err := s.repos.Modules.Get(ctx, n.GetOwner(), n.GetModule())
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
		}
		// Access check for private modules
		if !s.repos.Auth.CanReadModule(ctx, userID(caller), m) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
		}
		l, err := s.repos.Labels.Get(ctx, m.ID, n.GetLabel())
		if err != nil {
			if errors.Is(err, iface.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("label not found: "+n.GetLabel()))
			}
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		ownerID := resolveOwnerIDHelper(ctx, s.repos, m)
		labels = append(labels, labelToProto(l, ownerID))
	}
	return connect.NewResponse(&modulev1.GetLabelsResponse{Labels: labels}), nil
}

func (s *LabelService) ListLabels(
	ctx context.Context,
	req *connect.Request[modulev1.ListLabelsRequest],
) (*connect.Response[modulev1.ListLabelsResponse], error) {
	caller := auth.UserFromContext(ctx)
	ref := req.Msg.GetResourceRef()
	m, _, err := resolveResourceRef(ctx, s.repos, ref)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	// Access check
	if !s.repos.Auth.CanReadModule(ctx, userID(caller), m) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
	}

	dbLabels, err := s.repos.Labels.List(ctx, m.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	ownerID := resolveOwnerIDHelper(ctx, s.repos, m)
	var labels []*modulev1.Label
	for _, l := range dbLabels {
		labels = append(labels, labelToProto(l, ownerID))
	}
	return connect.NewResponse(&modulev1.ListLabelsResponse{Labels: labels}), nil
}

func (s *LabelService) CreateOrUpdateLabels(
	ctx context.Context,
	req *connect.Request[modulev1.CreateOrUpdateLabelsRequest],
) (*connect.Response[modulev1.CreateOrUpdateLabelsResponse], error) {
	caller := auth.UserFromContext(ctx)
	if caller == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	var labels []*modulev1.Label
	for _, v := range req.Msg.GetValues() {
		ref := v.GetLabelRef()
		if ref == nil || ref.GetName() == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("label ref with name required"))
		}
		n := ref.GetName()
		m, err := s.repos.Modules.Get(ctx, n.GetOwner(), n.GetModule())
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
		}

		if !s.repos.Auth.CanAdminModule(ctx, caller.ID, m) {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("no admin access"))
		}

		// Validate commit belongs to this module
		if _, err := s.repos.Commits.GetByID(ctx, m.ID, v.GetCommitId()); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("commit not found in module"))
		}

		now := time.Now().UTC()
		l := &model.Label{
			ID:        uuid.NewString(),
			ModuleID:  m.ID,
			Name:      n.GetLabel(),
			CommitID:  v.GetCommitId(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.repos.Labels.Upsert(ctx, l); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to upsert label"))
		}

		ownerID := resolveOwnerIDHelper(ctx, s.repos, m)
		labels = append(labels, labelToProto(l, ownerID))
	}
	return connect.NewResponse(&modulev1.CreateOrUpdateLabelsResponse{Labels: labels}), nil
}

func labelToProto(l *model.Label, ownerID string) *modulev1.Label {
	return &modulev1.Label{
		Id:         l.ID,
		Name:       l.Name,
		ModuleId:   l.ModuleID,
		OwnerId:    ownerID,
		CommitId:   l.CommitID,
		CreateTime: timestamppb.New(l.CreatedAt),
		UpdateTime: timestamppb.New(l.UpdatedAt),
	}
}
