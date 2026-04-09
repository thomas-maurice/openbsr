package authz

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

// --- fakes ---

type fakeUserRepo struct{ users map[string]*model.User }

func (f *fakeUserRepo) Create(_ context.Context, u *model.User) error {
	f.users[u.ID] = u; return nil
}
func (f *fakeUserRepo) GetByUsername(_ context.Context, username string) (*model.User, error) {
	for _, u := range f.users { if u.Username == username { return u, nil } }
	return nil, iface.ErrNotFound
}
func (f *fakeUserRepo) GetByID(_ context.Context, id string) (*model.User, error) {
	u, ok := f.users[id]; if !ok { return nil, iface.ErrNotFound }; return u, nil
}
func (f *fakeUserRepo) ListByOrg(_ context.Context, orgID string) ([]*model.User, error) {
	return nil, nil
}

type fakeOrgRepo struct {
	orgs    map[string]*model.Org
	members []*model.OrgMember
}

func (f *fakeOrgRepo) Create(_ context.Context, o *model.Org) error { f.orgs[o.ID] = o; return nil }
func (f *fakeOrgRepo) GetByName(_ context.Context, name string) (*model.Org, error) {
	for _, o := range f.orgs { if o.Name == name { return o, nil } }
	return nil, iface.ErrNotFound
}
func (f *fakeOrgRepo) GetByID(_ context.Context, id string) (*model.Org, error) {
	o, ok := f.orgs[id]; if !ok { return nil, iface.ErrNotFound }; return o, nil
}
func (f *fakeOrgRepo) AddMember(_ context.Context, orgID, userID string, role model.OrgRole) error {
	f.members = append(f.members, &model.OrgMember{OrgID: orgID, UserID: userID, Role: role}); return nil
}
func (f *fakeOrgRepo) RemoveMember(_ context.Context, orgID, userID string) error { return nil }
func (f *fakeOrgRepo) ListMembers(_ context.Context, orgID string) ([]*model.OrgMember, error) {
	return nil, nil
}
func (f *fakeOrgRepo) GetMember(_ context.Context, orgID, userID string) (*model.OrgMember, error) {
	for _, m := range f.members { if m.OrgID == orgID && m.UserID == userID { return m, nil } }
	return nil, iface.ErrNotFound
}

func setup(t *testing.T) (*Authorizer, *iface.Repos) {
	t.Helper()
	repos := &iface.Repos{
		Users: &fakeUserRepo{users: map[string]*model.User{
			"alice": {ID: "alice", Username: "alice", CreatedAt: time.Now()},
			"bob":   {ID: "bob", Username: "bob", CreatedAt: time.Now()},
			"eve":   {ID: "eve", Username: "eve", CreatedAt: time.Now()},
		}},
		Orgs: &fakeOrgRepo{
			orgs: map[string]*model.Org{
				"org1": {ID: "org1", Name: "myorg", CreatedAt: time.Now()},
			},
			members: []*model.OrgMember{
				{OrgID: "org1", UserID: "alice", Role: model.OrgRoleAdmin},
				{OrgID: "org1", UserID: "bob", Role: model.OrgRoleMember},
			},
		},
	}
	az, err := New(repos)
	if err != nil {
		t.Fatal(err)
	}
	return az, repos
}

var ctx = context.Background()

// ===================== Module Read =====================

func TestCanReadModule_PublicAnyone(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "alice", OwnerType: model.OwnerTypeUser, Visibility: model.VisibilityPublic}
	if !az.CanReadModule(ctx, "", m) {
		t.Fatal("public module should be readable by anyone")
	}
}

func TestCanReadModule_PrivateOwner(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "alice", OwnerType: model.OwnerTypeUser, Visibility: model.VisibilityPrivate}
	if !az.CanReadModule(ctx, "alice", m) {
		t.Fatal("owner should read private module")
	}
}

func TestCanReadModule_PrivateStranger(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "alice", OwnerType: model.OwnerTypeUser, Visibility: model.VisibilityPrivate}
	if az.CanReadModule(ctx, "eve", m) {
		t.Fatal("stranger should NOT read private user module")
	}
}

func TestCanReadModule_PrivateUnauthenticated(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "alice", OwnerType: model.OwnerTypeUser, Visibility: model.VisibilityPrivate}
	if az.CanReadModule(ctx, "", m) {
		t.Fatal("unauthenticated should NOT read private module")
	}
}

func TestCanReadModule_PrivateOrgMember(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "myorg", OwnerType: model.OwnerTypeOrg, Visibility: model.VisibilityPrivate}
	if !az.CanReadModule(ctx, "bob", m) {
		t.Fatal("org member should read private org module")
	}
}

func TestCanReadModule_PrivateOrgNonMember(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "myorg", OwnerType: model.OwnerTypeOrg, Visibility: model.VisibilityPrivate}
	if az.CanReadModule(ctx, "eve", m) {
		t.Fatal("non-member should NOT read private org module")
	}
}

// ===================== Module Write =====================

func TestCanWriteModule_Owner(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "alice", OwnerType: model.OwnerTypeUser}
	if !az.CanWriteModule(ctx, "alice", m) {
		t.Fatal("owner should write")
	}
}

func TestCanWriteModule_Stranger(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "alice", OwnerType: model.OwnerTypeUser}
	if az.CanWriteModule(ctx, "eve", m) {
		t.Fatal("stranger should NOT write")
	}
}

func TestCanWriteModule_OrgAdmin(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "myorg", OwnerType: model.OwnerTypeOrg}
	if !az.CanWriteModule(ctx, "alice", m) {
		t.Fatal("org admin should write")
	}
}

func TestCanWriteModule_OrgMember(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "myorg", OwnerType: model.OwnerTypeOrg}
	if !az.CanWriteModule(ctx, "bob", m) {
		t.Fatal("org member should write")
	}
}

func TestCanWriteModule_OrgNonMember(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "myorg", OwnerType: model.OwnerTypeOrg}
	if az.CanWriteModule(ctx, "eve", m) {
		t.Fatal("non-member should NOT write")
	}
}

func TestCanWriteModule_Unauthenticated(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "alice", OwnerType: model.OwnerTypeUser}
	if az.CanWriteModule(ctx, "", m) {
		t.Fatal("unauthenticated should NOT write")
	}
}

// ===================== Module Admin =====================

func TestCanAdminModule_Owner(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "alice", OwnerType: model.OwnerTypeUser}
	if !az.CanAdminModule(ctx, "alice", m) {
		t.Fatal("owner should admin")
	}
}

func TestCanAdminModule_OrgAdmin(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "myorg", OwnerType: model.OwnerTypeOrg}
	if !az.CanAdminModule(ctx, "alice", m) {
		t.Fatal("org admin should admin")
	}
}

func TestCanAdminModule_OrgMemberCannotAdmin(t *testing.T) {
	az, _ := setup(t)
	m := &model.Module{OwnerName: "myorg", OwnerType: model.OwnerTypeOrg}
	if az.CanAdminModule(ctx, "bob", m) {
		t.Fatal("org member should NOT admin")
	}
}

// ===================== Create Module =====================

func TestCanCreateModule_UnderSelf(t *testing.T) {
	az, _ := setup(t)
	if !az.CanCreateModule(ctx, "alice", "alice") {
		t.Fatal("user should create under self")
	}
}

func TestCanCreateModule_UnderOrgAdmin(t *testing.T) {
	az, _ := setup(t)
	if !az.CanCreateModule(ctx, "alice", "myorg") {
		t.Fatal("org admin should create under org")
	}
}

func TestCanCreateModule_UnderOrgMemberDenied(t *testing.T) {
	az, _ := setup(t)
	if az.CanCreateModule(ctx, "bob", "myorg") {
		t.Fatal("org member should NOT create modules")
	}
}

func TestCanCreateModule_UnderOtherUserDenied(t *testing.T) {
	az, _ := setup(t)
	if az.CanCreateModule(ctx, "eve", "alice") {
		t.Fatal("should NOT create under other user")
	}
}

func TestCanCreateModule_Unauthenticated(t *testing.T) {
	az, _ := setup(t)
	if az.CanCreateModule(ctx, "", "alice") {
		t.Fatal("unauthenticated should NOT create")
	}
}

// ===================== Org Members =====================

func TestCanManageOrgMembers_Admin(t *testing.T) {
	az, _ := setup(t)
	if !az.CanManageOrgMembers(ctx, "alice", "org1") {
		t.Fatal("org admin should manage members")
	}
}

func TestCanManageOrgMembers_MemberDenied(t *testing.T) {
	az, _ := setup(t)
	if az.CanManageOrgMembers(ctx, "bob", "org1") {
		t.Fatal("org member should NOT manage members")
	}
}

func TestCanManageOrgMembers_NonMemberDenied(t *testing.T) {
	az, _ := setup(t)
	if az.CanManageOrgMembers(ctx, "eve", "org1") {
		t.Fatal("non-member should NOT manage members")
	}
}

func TestCanManageOrgMembers_UnauthenticatedDenied(t *testing.T) {
	az, _ := setup(t)
	if az.CanManageOrgMembers(ctx, "", "org1") {
		t.Fatal("unauthenticated should NOT manage members")
	}
}

// ===================== Policy Summary =====================

func TestPolicySummary(t *testing.T) {
	s := PolicySummary()
	if s == "" {
		t.Fatal("policy summary should not be empty")
	}
	if !strings.Contains(s, "owner can read module") {
		t.Fatal("should contain owner read policy")
	}
	if !strings.Contains(s, "org_member can write module") {
		t.Fatal("should contain org_member write policy")
	}
	if !strings.Contains(s, "org_admin can manage_members org") {
		t.Fatal("should contain org_admin manage_members policy")
	}
}
