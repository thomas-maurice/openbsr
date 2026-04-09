package iface

import (
	"context"
	"errors"

	"github.com/thomas-maurice/openbsr/internal/model"
)

var ErrNotFound = errors.New("not found")

type UserRepo interface {
	Create(ctx context.Context, u *model.User) error
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
	ListByOrg(ctx context.Context, orgID string) ([]*model.User, error)
}

type OrgRepo interface {
	Create(ctx context.Context, o *model.Org) error
	GetByName(ctx context.Context, name string) (*model.Org, error)
	GetByID(ctx context.Context, id string) (*model.Org, error)
	AddMember(ctx context.Context, orgID, userID string, role model.OrgRole) error
	RemoveMember(ctx context.Context, orgID, userID string) error
	ListMembers(ctx context.Context, orgID string) ([]*model.OrgMember, error)
	GetMember(ctx context.Context, orgID, userID string) (*model.OrgMember, error)
}

type ModuleRepo interface {
	Create(ctx context.Context, m *model.Module) error
	Get(ctx context.Context, ownerName, repoName string) (*model.Module, error)
	GetByID(ctx context.Context, id string) (*model.Module, error)
	List(ctx context.Context, ownerName string) ([]*model.Module, error)
	ListPublic(ctx context.Context, query string, limit int) ([]*model.Module, error)
	SetVisibility(ctx context.Context, id string, vis model.Visibility) error
	Delete(ctx context.Context, id string) error
}

type CommitRepo interface {
	Create(ctx context.Context, c *model.Commit) error
	GetByID(ctx context.Context, moduleID, commitID string) (*model.Commit, error)
	GetLatest(ctx context.Context, moduleID string) (*model.Commit, error)
	ListByModule(ctx context.Context, moduleID string) ([]*model.Commit, error)
}

type LabelRepo interface {
	Upsert(ctx context.Context, l *model.Label) error
	Get(ctx context.Context, moduleID, name string) (*model.Label, error)
	List(ctx context.Context, moduleID string) ([]*model.Label, error)
}

type TokenRepo interface {
	Create(ctx context.Context, t *model.Token) error
	GetByHash(ctx context.Context, hash string) (*model.Token, error)
	ListByUser(ctx context.Context, userID string) ([]*model.Token, error)
	Revoke(ctx context.Context, id, userID string) error
	DeleteExpired(ctx context.Context) error
	DeleteByUserAndNote(ctx context.Context, userID, note string) error
}

// Authorizer provides centralized authorization checks.
// Implemented by internal/authz.Authorizer.
type Authorizer interface {
	CanReadModule(ctx context.Context, userID string, m *model.Module) bool
	CanWriteModule(ctx context.Context, userID string, m *model.Module) bool
	CanAdminModule(ctx context.Context, userID string, m *model.Module) bool
	CanCreateModule(ctx context.Context, userID, ownerName string) bool
	CanManageOrgMembers(ctx context.Context, userID, orgID string) bool
}

type Repos struct {
	Users   UserRepo
	Orgs    OrgRepo
	Modules ModuleRepo
	Commits CommitRepo
	Labels  LabelRepo
	Tokens  TokenRepo
	Auth    Authorizer
}
