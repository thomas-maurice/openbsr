package module

import (
	"context"
	"errors"
	"regexp"
	"time"

	"connectrpc.com/connect"
	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	ownerv1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/owner/v1"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/module/v1/modulev1connect"
	"github.com/google/uuid"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var moduleNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,37}[a-z0-9]$`)

type ModuleService struct {
	modulev1connect.UnimplementedModuleServiceHandler
	repos *iface.Repos
}

func NewModuleService(repos *iface.Repos) *ModuleService {
	return &ModuleService{repos: repos}
}

func (s *ModuleService) GetModules(
	ctx context.Context,
	req *connect.Request[modulev1.GetModulesRequest],
) (*connect.Response[modulev1.GetModulesResponse], error) {
	caller := auth.UserFromContext(ctx)
	var modules []*modulev1.Module
	for _, ref := range req.Msg.GetModuleRefs() {
		var m *model.Module
		var err error
		if ref.GetName() != nil {
			m, err = s.repos.Modules.Get(ctx, ref.GetName().GetOwner(), ref.GetName().GetModule())
		} else if ref.GetId() != "" {
			m, err = s.repos.Modules.GetByID(ctx, ref.GetId())
		} else {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("module ref must have name or id"))
		}
		if err != nil {
			if errors.Is(err, iface.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		if !s.repos.Auth.CanReadModule(ctx, userID(caller), m) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("module not found"))
		}
		modules = append(modules, moduleToProto(m, resolveOwnerIDHelper(ctx, s.repos, m)))
	}
	return connect.NewResponse(&modulev1.GetModulesResponse{Modules: modules}), nil
}

func (s *ModuleService) ListModules(
	ctx context.Context,
	req *connect.Request[modulev1.ListModulesRequest],
) (*connect.Response[modulev1.ListModulesResponse], error) {
	caller := auth.UserFromContext(ctx)
	var allModules []*modulev1.Module
	for _, ownerRef := range req.Msg.GetOwnerRefs() {
		ownerName := resolveOwnerRefName(ownerRef)
		if ownerName == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("owner ref must have name"))
		}
		mods, err := s.repos.Modules.List(ctx, ownerName)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		for _, m := range mods {
			if !s.repos.Auth.CanReadModule(ctx, userID(caller), m) {
				continue
			}
			allModules = append(allModules, moduleToProto(m, resolveOwnerIDHelper(ctx, s.repos, m)))
		}
	}
	return connect.NewResponse(&modulev1.ListModulesResponse{Modules: allModules}), nil
}

func (s *ModuleService) CreateModules(
	ctx context.Context,
	req *connect.Request[modulev1.CreateModulesRequest],
) (*connect.Response[modulev1.CreateModulesResponse], error) {
	caller := auth.UserFromContext(ctx)
	if caller == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	var modules []*modulev1.Module
	for _, v := range req.Msg.GetValues() {
		ownerName := resolveOwnerRefName(v.GetOwnerRef())
		if !s.repos.Auth.CanCreateModule(ctx, caller.ID, ownerName) {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot create modules under: "+ownerName))
		}
		ownerType := resolveOwnerTypeFromName(ctx, s.repos, caller, ownerName)
		if !moduleNameRe.MatchString(v.GetName()) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("module name must be 2-39 chars, lowercase alphanumeric and hyphens"))
		}
		vis := model.VisibilityPrivate
		if v.GetVisibility() == modulev1.ModuleVisibility_MODULE_VISIBILITY_PUBLIC {
			vis = model.VisibilityPublic
		}
		// Check for existing module first
		if _, lookupErr := s.repos.Modules.Get(ctx, ownerName, v.GetName()); lookupErr == nil {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("module already exists: "+ownerName+"/"+v.GetName()))
		}
		m := &model.Module{
			ID:         uuid.NewString(),
			OwnerName:  ownerName,
			OwnerType:  ownerType,
			Name:       v.GetName(),
			Visibility: vis,
			CreatedAt:  time.Now().UTC(),
		}
		if err := s.repos.Modules.Create(ctx, m); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		modules = append(modules, moduleToProto(m, resolveOwnerIDHelper(ctx, s.repos, m)))
	}
	return connect.NewResponse(&modulev1.CreateModulesResponse{Modules: modules}), nil
}

func resolveOwnerRefName(ref *ownerv1.OwnerRef) string {
	if ref == nil {
		return ""
	}
	return ref.GetName()
}

// resolveOwnerTypeFromName returns the OwnerType for a given owner name (no authz — authz is done by caller).
func resolveOwnerTypeFromName(ctx context.Context, repos *iface.Repos, caller *model.User, ownerName string) model.OwnerType {
	if ownerName == caller.Username {
		return model.OwnerTypeUser
	}
	return model.OwnerTypeOrg
}

// userID extracts user ID from a model.User, returning "" for nil.
func userID(u *model.User) string {
	if u == nil {
		return ""
	}
	return u.ID
}

func resolveOwnerIDHelper(ctx context.Context, repos *iface.Repos, m *model.Module) string {
	if m.OwnerType == model.OwnerTypeUser {
		u, err := repos.Users.GetByUsername(ctx, m.OwnerName)
		if err == nil {
			return u.ID
		}
	} else {
		o, err := repos.Orgs.GetByName(ctx, m.OwnerName)
		if err == nil {
			return o.ID
		}
	}
	return ""
}

func moduleToProto(m *model.Module, ownerID string) *modulev1.Module {
	vis := modulev1.ModuleVisibility_MODULE_VISIBILITY_PRIVATE
	if m.Visibility == model.VisibilityPublic {
		vis = modulev1.ModuleVisibility_MODULE_VISIBILITY_PUBLIC
	}
	return &modulev1.Module{
		Id:               m.ID,
		Name:             m.Name,
		OwnerId:          ownerID,
		Visibility:       vis,
		State:            modulev1.ModuleState_MODULE_STATE_ACTIVE,
		CreateTime:       timestamppb.New(m.CreatedAt),
		UpdateTime:       timestamppb.New(m.CreatedAt),
		DefaultLabelName: "main",
	}
}
