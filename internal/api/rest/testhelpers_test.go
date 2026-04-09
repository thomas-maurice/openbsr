package rest

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thomas-maurice/openbsr/internal/authz"

	"github.com/google/uuid"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

// --- fake OrgRepo ---

type fakeOrgRepo struct {
	orgs    map[string]*model.Org
	members []*model.OrgMember
}

func (f *fakeOrgRepo) Create(_ context.Context, o *model.Org) error {
	for _, existing := range f.orgs {
		if existing.Name == o.Name {
			return iface.ErrNotFound
		}
	}
	f.orgs[o.ID] = o
	return nil
}
func (f *fakeOrgRepo) GetByName(_ context.Context, name string) (*model.Org, error) {
	for _, o := range f.orgs {
		if o.Name == name {
			return o, nil
		}
	}
	return nil, iface.ErrNotFound
}
func (f *fakeOrgRepo) GetByID(_ context.Context, id string) (*model.Org, error) {
	o, ok := f.orgs[id]
	if !ok {
		return nil, iface.ErrNotFound
	}
	return o, nil
}
func (f *fakeOrgRepo) AddMember(_ context.Context, orgID, userID string, role model.OrgRole) error {
	f.members = append(f.members, &model.OrgMember{OrgID: orgID, UserID: userID, Role: role})
	return nil
}
func (f *fakeOrgRepo) RemoveMember(_ context.Context, orgID, userID string) error {
	for i, m := range f.members {
		if m.OrgID == orgID && m.UserID == userID {
			f.members = append(f.members[:i], f.members[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeOrgRepo) ListMembers(_ context.Context, orgID string) ([]*model.OrgMember, error) {
	var result []*model.OrgMember
	for _, m := range f.members {
		if m.OrgID == orgID {
			result = append(result, m)
		}
	}
	return result, nil
}
func (f *fakeOrgRepo) GetMember(_ context.Context, orgID, userID string) (*model.OrgMember, error) {
	for _, m := range f.members {
		if m.OrgID == orgID && m.UserID == userID {
			return m, nil
		}
	}
	return nil, iface.ErrNotFound
}

// --- fake ModuleRepo ---

type fakeModuleRepo struct {
	modules map[string]*model.Module
}

func (f *fakeModuleRepo) Create(_ context.Context, m *model.Module) error {
	for _, existing := range f.modules {
		if existing.OwnerName == m.OwnerName && existing.Name == m.Name {
			return iface.ErrNotFound
		}
	}
	f.modules[m.ID] = m
	return nil
}
func (f *fakeModuleRepo) Get(_ context.Context, ownerName, repoName string) (*model.Module, error) {
	for _, m := range f.modules {
		if m.OwnerName == ownerName && m.Name == repoName {
			return m, nil
		}
	}
	return nil, iface.ErrNotFound
}
func (f *fakeModuleRepo) GetByID(_ context.Context, id string) (*model.Module, error) {
	m, ok := f.modules[id]
	if !ok {
		return nil, iface.ErrNotFound
	}
	return m, nil
}
func (f *fakeModuleRepo) List(_ context.Context, ownerName string) ([]*model.Module, error) {
	var result []*model.Module
	for _, m := range f.modules {
		if m.OwnerName == ownerName {
			result = append(result, m)
		}
	}
	return result, nil
}
func (f *fakeModuleRepo) ListPublic(_ context.Context, query string, limit int) ([]*model.Module, error) {
	var result []*model.Module
	for _, m := range f.modules {
		if m.Visibility != model.VisibilityPublic {
			continue
		}
		if query != "" && !strings.Contains(m.Name, query) && !strings.Contains(m.OwnerName, query) {
			continue
		}
		result = append(result, m)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (f *fakeModuleRepo) SetVisibility(_ context.Context, id string, vis model.Visibility) error {
	m, ok := f.modules[id]
	if !ok {
		return iface.ErrNotFound
	}
	m.Visibility = vis
	return nil
}
func (f *fakeModuleRepo) Delete(_ context.Context, id string) error {
	delete(f.modules, id)
	return nil
}

// Use the real Casbin authorizer for handler tests so authz behavior is validated.

// --- shared setup ---

func setupM2Handler(t *testing.T) (*http.ServeMux, *iface.Repos) {
	t.Helper()
	repos := &iface.Repos{
		Users:   &fakeUserRepo{users: make(map[string]*model.User)},
		Tokens:  &fakeTokenRepo{tokens: make(map[string]*model.Token)},
		Orgs:    &fakeOrgRepo{orgs: make(map[string]*model.Org), members: nil},
		Modules: &fakeModuleRepo{modules: make(map[string]*model.Module)},
	}
	az, err := authz.New(repos)
	if err != nil {
		t.Fatal(err)
	}
	repos.Auth = az
	mux := http.NewServeMux()
	return mux, repos
}

func createTestUser(t *testing.T, repos *iface.Repos, username string) *model.User {
	t.Helper()
	hash, _ := auth.HashPassword("testpassword")
	u := &model.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := repos.Users.Create(context.Background(), u); err != nil {
		t.Fatalf("failed to create test user %s: %v", username, err)
	}
	return u
}
