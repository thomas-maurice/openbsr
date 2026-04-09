package owner

import (
	"context"
	"errors"
	"regexp"
	"time"

	"connectrpc.com/connect"
	ownerv1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/owner/v1"
	"buf.build/gen/go/bufbuild/registry/connectrpc/go/buf/registry/owner/v1/ownerv1connect"
	"github.com/google/uuid"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,37}[a-z0-9]$`)

// --- OwnerService ---

type OwnerService struct {
	ownerv1connect.UnimplementedOwnerServiceHandler
	repos *iface.Repos
}

func NewOwnerService(repos *iface.Repos) *OwnerService {
	return &OwnerService{repos: repos}
}

func (s *OwnerService) GetOwners(
	ctx context.Context,
	req *connect.Request[ownerv1.GetOwnersRequest],
) (*connect.Response[ownerv1.GetOwnersResponse], error) {
	var owners []*ownerv1.Owner
	for _, ref := range req.Msg.GetOwnerRefs() {
		name := ref.GetName()
		id := ref.GetId()
		// Try user first, then org
		if name != "" {
			u, uErr := s.repos.Users.GetByUsername(ctx, name)
			if uErr == nil {
				owners = append(owners, &ownerv1.Owner{Value: &ownerv1.Owner_User{User: userToProto(u)}})
				continue
			}
			if !errors.Is(uErr, iface.ErrNotFound) {
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
			}
			o, oErr := s.repos.Orgs.GetByName(ctx, name)
			if oErr == nil {
				owners = append(owners, &ownerv1.Owner{Value: &ownerv1.Owner_Organization{Organization: orgToProto(o)}})
				continue
			}
			if !errors.Is(oErr, iface.ErrNotFound) {
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
			}
			return nil, connect.NewError(connect.CodeNotFound, errors.New("owner not found: "+name))
		}
		if id != "" {
			u, uErr := s.repos.Users.GetByID(ctx, id)
			if uErr == nil {
				owners = append(owners, &ownerv1.Owner{Value: &ownerv1.Owner_User{User: userToProto(u)}})
				continue
			}
			if !errors.Is(uErr, iface.ErrNotFound) {
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
			}
			o, oErr := s.repos.Orgs.GetByID(ctx, id)
			if oErr == nil {
				owners = append(owners, &ownerv1.Owner{Value: &ownerv1.Owner_Organization{Organization: orgToProto(o)}})
				continue
			}
			if !errors.Is(oErr, iface.ErrNotFound) {
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
			}
			return nil, connect.NewError(connect.CodeNotFound, errors.New("owner not found: "+id))
		}
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("owner ref must have name or id"))
	}
	return connect.NewResponse(&ownerv1.GetOwnersResponse{Owners: owners}), nil
}

// --- OrganizationService ---

type OrganizationService struct {
	ownerv1connect.UnimplementedOrganizationServiceHandler
	repos *iface.Repos
}

func NewOrganizationService(repos *iface.Repos) *OrganizationService {
	return &OrganizationService{repos: repos}
}

func (s *OrganizationService) GetOrganizations(
	ctx context.Context,
	req *connect.Request[ownerv1.GetOrganizationsRequest],
) (*connect.Response[ownerv1.GetOrganizationsResponse], error) {
	var orgs []*ownerv1.Organization
	for _, ref := range req.Msg.GetOrganizationRefs() {
		var o *model.Org
		var err error
		if ref.GetName() != "" {
			o, err = s.repos.Orgs.GetByName(ctx, ref.GetName())
		} else if ref.GetId() != "" {
			o, err = s.repos.Orgs.GetByID(ctx, ref.GetId())
		} else {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("organization ref must have name or id"))
		}
		if err != nil {
			if errors.Is(err, iface.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		orgs = append(orgs, orgToProto(o))
	}
	return connect.NewResponse(&ownerv1.GetOrganizationsResponse{Organizations: orgs}), nil
}

func (s *OrganizationService) CreateOrganizations(
	ctx context.Context,
	req *connect.Request[ownerv1.CreateOrganizationsRequest],
) (*connect.Response[ownerv1.CreateOrganizationsResponse], error) {
	u := auth.UserFromContext(ctx)
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	var orgs []*ownerv1.Organization
	for _, v := range req.Msg.GetValues() {
		if !nameRe.MatchString(v.GetName()) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("org name must be 3-39 chars, lowercase alphanumeric and hyphens"))
		}
		// Check for existing org first
		if _, lookupErr := s.repos.Orgs.GetByName(ctx, v.GetName()); lookupErr == nil {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("organization already exists: "+v.GetName()))
		}
		o := &model.Org{
			ID:        uuid.NewString(),
			Name:      v.GetName(),
			CreatedAt: time.Now().UTC(),
		}
		if err := s.repos.Orgs.Create(ctx, o); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		if err := s.repos.Orgs.AddMember(ctx, o.ID, u.ID, model.OrgRoleAdmin); err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to add admin member"))
		}
		orgs = append(orgs, orgToProto(o))
	}
	return connect.NewResponse(&ownerv1.CreateOrganizationsResponse{Organizations: orgs}), nil
}

// --- UserService ---

type UserService struct {
	ownerv1connect.UnimplementedUserServiceHandler
	repos   *iface.Repos
	openReg bool
}

func NewUserService(repos *iface.Repos, openReg bool) *UserService {
	return &UserService{repos: repos, openReg: openReg}
}

func (s *UserService) GetUsers(
	ctx context.Context,
	req *connect.Request[ownerv1.GetUsersRequest],
) (*connect.Response[ownerv1.GetUsersResponse], error) {
	var users []*ownerv1.User
	for _, ref := range req.Msg.GetUserRefs() {
		var u *model.User
		var err error
		if ref.GetName() != "" {
			u, err = s.repos.Users.GetByUsername(ctx, ref.GetName())
		} else if ref.GetId() != "" {
			u, err = s.repos.Users.GetByID(ctx, ref.GetId())
		} else {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user ref must have name or id"))
		}
		if err != nil {
			if errors.Is(err, iface.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		users = append(users, userToProto(u))
	}
	return connect.NewResponse(&ownerv1.GetUsersResponse{Users: users}), nil
}

func (s *UserService) GetCurrentUser(
	ctx context.Context,
	req *connect.Request[ownerv1.GetCurrentUserRequest],
) (*connect.Response[ownerv1.GetCurrentUserResponse], error) {
	u := auth.UserFromContext(ctx)
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	return connect.NewResponse(&ownerv1.GetCurrentUserResponse{User: userToProto(u)}), nil
}

func (s *UserService) CreateUsers(
	ctx context.Context,
	req *connect.Request[ownerv1.CreateUsersRequest],
) (*connect.Response[ownerv1.CreateUsersResponse], error) {
	if !s.openReg {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("registration is closed"))
	}
	var users []*ownerv1.User
	for _, v := range req.Msg.GetValues() {
		if !nameRe.MatchString(v.GetName()) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("username must be 3-39 chars, lowercase alphanumeric and hyphens"))
		}
		u := &model.User{
			ID:        uuid.NewString(),
			Username:  v.GetName(),
			CreatedAt: time.Now().UTC(),
		}
		if err := s.repos.Users.Create(ctx, u); err != nil {
			if _, lookupErr := s.repos.Users.GetByUsername(ctx, v.GetName()); lookupErr == nil {
				return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("user already exists: "+v.GetName()))
			}
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
		}
		users = append(users, userToProto(u))
	}
	return connect.NewResponse(&ownerv1.CreateUsersResponse{Users: users}), nil
}

// --- helpers ---

func userToProto(u *model.User) *ownerv1.User {
	return &ownerv1.User{
		Id:         u.ID,
		Name:       u.Username,
		CreateTime: timestamppb.New(u.CreatedAt),
		UpdateTime: timestamppb.New(u.CreatedAt),
		State:      ownerv1.UserState_USER_STATE_ACTIVE,
		Type:       ownerv1.UserType_USER_TYPE_STANDARD,
	}
}

func orgToProto(o *model.Org) *ownerv1.Organization {
	return &ownerv1.Organization{
		Id:         o.ID,
		Name:       o.Name,
		CreateTime: timestamppb.New(o.CreatedAt),
		UpdateTime: timestamppb.New(o.CreatedAt),
	}
}
