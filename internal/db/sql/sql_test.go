package sql

import (
	"context"
	"testing"
	"time"

	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

func setupTestDB(t *testing.T) *iface.Repos {
	t.Helper()
	db, err := Connect("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db.Repos()
}

var ctx = context.Background()

// ===================== UserStore =====================

func TestSQLUser_CreateAndGet(t *testing.T) {
	repos := setupTestDB(t)
	u := &model.User{ID: "u1", Username: "alice", PasswordHash: "hash", CreatedAt: time.Now().UTC()}
	if err := repos.Users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := repos.Users.GetByUsername(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "u1" {
		t.Fatalf("expected u1, got %s", got.ID)
	}
	got2, err := repos.Users.GetByID(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got2.Username != "alice" {
		t.Fatalf("expected alice, got %s", got2.Username)
	}
}

func TestSQLUser_DuplicateUsername(t *testing.T) {
	repos := setupTestDB(t)
	repos.Users.Create(ctx, &model.User{ID: "u1", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	err := repos.Users.Create(ctx, &model.User{ID: "u2", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestSQLUser_NotFound(t *testing.T) {
	repos := setupTestDB(t)
	_, err := repos.Users.GetByUsername(ctx, "nope")
	if err != iface.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLUser_ListByOrg(t *testing.T) {
	repos := setupTestDB(t)
	repos.Users.Create(ctx, &model.User{ID: "u1", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	repos.Users.Create(ctx, &model.User{ID: "u2", Username: "bob", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	repos.Orgs.Create(ctx, &model.Org{ID: "o1", Name: "myorg", CreatedAt: time.Now().UTC()})
	repos.Orgs.AddMember(ctx, "o1", "u1", model.OrgRoleAdmin)
	repos.Orgs.AddMember(ctx, "o1", "u2", model.OrgRoleMember)
	users, err := repos.Users.ListByOrg(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

// ===================== OrgStore =====================

func TestSQLOrg_CreateAndGet(t *testing.T) {
	repos := setupTestDB(t)
	repos.Orgs.Create(ctx, &model.Org{ID: "o1", Name: "myorg", CreatedAt: time.Now().UTC()})
	got, err := repos.Orgs.GetByName(ctx, "myorg")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "o1" {
		t.Fatalf("expected o1, got %s", got.ID)
	}
}

func TestSQLOrg_DuplicateName(t *testing.T) {
	repos := setupTestDB(t)
	repos.Orgs.Create(ctx, &model.Org{ID: "o1", Name: "myorg", CreatedAt: time.Now().UTC()})
	err := repos.Orgs.Create(ctx, &model.Org{ID: "o2", Name: "myorg", CreatedAt: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestSQLOrg_Members(t *testing.T) {
	repos := setupTestDB(t)
	repos.Users.Create(ctx, &model.User{ID: "u1", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	repos.Orgs.Create(ctx, &model.Org{ID: "o1", Name: "myorg", CreatedAt: time.Now().UTC()})
	repos.Orgs.AddMember(ctx, "o1", "u1", model.OrgRoleAdmin)

	m, err := repos.Orgs.GetMember(ctx, "o1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Role != model.OrgRoleAdmin {
		t.Fatalf("expected admin, got %s", m.Role)
	}

	members, _ := repos.Orgs.ListMembers(ctx, "o1")
	if len(members) != 1 {
		t.Fatalf("expected 1, got %d", len(members))
	}

	repos.Orgs.RemoveMember(ctx, "o1", "u1")
	_, err = repos.Orgs.GetMember(ctx, "o1", "u1")
	if err != iface.ErrNotFound {
		t.Fatalf("expected ErrNotFound after remove, got %v", err)
	}
}

// ===================== ModuleStore =====================

func TestSQLModule_CreateAndGet(t *testing.T) {
	repos := setupTestDB(t)
	repos.Modules.Create(ctx, &model.Module{ID: "m1", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "mod1", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	got, err := repos.Modules.Get(ctx, "alice", "mod1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "m1" {
		t.Fatalf("expected m1, got %s", got.ID)
	}
}

func TestSQLModule_DuplicateOwnerName(t *testing.T) {
	repos := setupTestDB(t)
	repos.Modules.Create(ctx, &model.Module{ID: "m1", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "mod1", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	err := repos.Modules.Create(ctx, &model.Module{ID: "m2", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "mod1", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestSQLModule_ListPublic(t *testing.T) {
	repos := setupTestDB(t)
	repos.Modules.Create(ctx, &model.Module{ID: "m1", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "pub", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	repos.Modules.Create(ctx, &model.Module{ID: "m2", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "priv", Visibility: model.VisibilityPrivate, CreatedAt: time.Now().UTC()})

	mods, _ := repos.Modules.ListPublic(ctx, "", 50)
	if len(mods) != 1 || mods[0].Name != "pub" {
		t.Fatalf("expected 1 public module, got %d", len(mods))
	}

	mods2, _ := repos.Modules.ListPublic(ctx, "pub", 50)
	if len(mods2) != 1 {
		t.Fatalf("search 'pub' expected 1, got %d", len(mods2))
	}

	mods3, _ := repos.Modules.ListPublic(ctx, "nonexistent", 50)
	if len(mods3) != 0 {
		t.Fatalf("search 'nonexistent' expected 0, got %d", len(mods3))
	}
}

func TestSQLModule_Delete(t *testing.T) {
	repos := setupTestDB(t)
	repos.Modules.Create(ctx, &model.Module{ID: "m1", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "mod1", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	repos.Modules.Delete(ctx, "m1")
	_, err := repos.Modules.GetByID(ctx, "m1")
	if err != iface.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ===================== CommitStore =====================

func TestSQLCommit_CreateAndGet(t *testing.T) {
	repos := setupTestDB(t)
	repos.Modules.Create(ctx, &model.Module{ID: "m1", OwnerName: "a", OwnerType: model.OwnerTypeUser, Name: "mod", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	repos.Commits.Create(ctx, &model.Commit{ID: "c1", ModuleID: "m1", StorageKey: "m1/c1", CreatedAt: time.Now().UTC()})
	got, err := repos.Commits.GetByID(ctx, "m1", "c1")
	if err != nil {
		t.Fatal(err)
	}
	if got.StorageKey != "m1/c1" {
		t.Fatalf("expected m1/c1, got %s", got.StorageKey)
	}
}

func TestSQLCommit_GetLatest(t *testing.T) {
	repos := setupTestDB(t)
	repos.Modules.Create(ctx, &model.Module{ID: "m1", OwnerName: "a", OwnerType: model.OwnerTypeUser, Name: "mod", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	t1 := time.Now().UTC().Add(-time.Hour)
	t2 := time.Now().UTC()
	repos.Commits.Create(ctx, &model.Commit{ID: "c1", ModuleID: "m1", StorageKey: "m1/c1", CreatedAt: t1})
	repos.Commits.Create(ctx, &model.Commit{ID: "c2", ModuleID: "m1", StorageKey: "m1/c2", CreatedAt: t2})
	got, _ := repos.Commits.GetLatest(ctx, "m1")
	if got.ID != "c2" {
		t.Fatalf("expected c2, got %s", got.ID)
	}
}

func TestSQLCommit_NotFound(t *testing.T) {
	repos := setupTestDB(t)
	_, err := repos.Commits.GetByID(ctx, "m1", "nope")
	if err != iface.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ===================== LabelStore =====================

func TestSQLLabel_UpsertAndGet(t *testing.T) {
	repos := setupTestDB(t)
	repos.Modules.Create(ctx, &model.Module{ID: "m1", OwnerName: "a", OwnerType: model.OwnerTypeUser, Name: "mod", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	repos.Commits.Create(ctx, &model.Commit{ID: "c1", ModuleID: "m1", StorageKey: "k", CreatedAt: time.Now().UTC()})
	now := time.Now().UTC()
	repos.Labels.Upsert(ctx, &model.Label{ID: "l1", ModuleID: "m1", Name: "main", CommitID: "c1", CreatedAt: now, UpdatedAt: now})
	got, err := repos.Labels.Get(ctx, "m1", "main")
	if err != nil {
		t.Fatal(err)
	}
	if got.CommitID != "c1" {
		t.Fatalf("expected c1, got %s", got.CommitID)
	}
}

func TestSQLLabel_UpsertUpdate(t *testing.T) {
	repos := setupTestDB(t)
	repos.Modules.Create(ctx, &model.Module{ID: "m1", OwnerName: "a", OwnerType: model.OwnerTypeUser, Name: "mod", Visibility: model.VisibilityPublic, CreatedAt: time.Now().UTC()})
	repos.Commits.Create(ctx, &model.Commit{ID: "c1", ModuleID: "m1", StorageKey: "k", CreatedAt: time.Now().UTC()})
	repos.Commits.Create(ctx, &model.Commit{ID: "c2", ModuleID: "m1", StorageKey: "k2", CreatedAt: time.Now().UTC()})
	now := time.Now().UTC()
	repos.Labels.Upsert(ctx, &model.Label{ID: "l1", ModuleID: "m1", Name: "main", CommitID: "c1", CreatedAt: now, UpdatedAt: now})
	repos.Labels.Upsert(ctx, &model.Label{ID: "l2", ModuleID: "m1", Name: "main", CommitID: "c2", CreatedAt: now, UpdatedAt: now})
	got, _ := repos.Labels.Get(ctx, "m1", "main")
	if got.CommitID != "c2" {
		t.Fatalf("expected c2 after upsert, got %s", got.CommitID)
	}
	// Should still be one label, not two
	labels, _ := repos.Labels.List(ctx, "m1")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label after upsert, got %d", len(labels))
	}
}

func TestSQLLabel_NotFound(t *testing.T) {
	repos := setupTestDB(t)
	_, err := repos.Labels.Get(ctx, "m1", "nope")
	if err != iface.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ===================== TokenStore =====================

func TestSQLToken_CreateAndGetByHash(t *testing.T) {
	repos := setupTestDB(t)
	repos.Users.Create(ctx, &model.User{ID: "u1", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	repos.Tokens.Create(ctx, &model.Token{ID: "t1", UserID: "u1", Hash: "abc", Note: "test", CreatedAt: time.Now().UTC()})
	got, err := repos.Tokens.GetByHash(ctx, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "t1" {
		t.Fatalf("expected t1, got %s", got.ID)
	}
}

func TestSQLToken_Revoke(t *testing.T) {
	repos := setupTestDB(t)
	repos.Users.Create(ctx, &model.User{ID: "u1", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	repos.Tokens.Create(ctx, &model.Token{ID: "t1", UserID: "u1", Hash: "abc", CreatedAt: time.Now().UTC()})
	if err := repos.Tokens.Revoke(ctx, "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	_, err := repos.Tokens.GetByHash(ctx, "abc")
	if err != iface.ErrNotFound {
		t.Fatalf("expected ErrNotFound after revoke, got %v", err)
	}
}

func TestSQLToken_RevokeWrongUser(t *testing.T) {
	repos := setupTestDB(t)
	repos.Users.Create(ctx, &model.User{ID: "u1", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	repos.Tokens.Create(ctx, &model.Token{ID: "t1", UserID: "u1", Hash: "abc", CreatedAt: time.Now().UTC()})
	err := repos.Tokens.Revoke(ctx, "t1", "wrong-user")
	if err != iface.ErrNotFound {
		t.Fatalf("expected ErrNotFound for wrong user, got %v", err)
	}
}

func TestSQLToken_DeleteExpired(t *testing.T) {
	repos := setupTestDB(t)
	repos.Users.Create(ctx, &model.User{ID: "u1", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	expired := time.Now().UTC().Add(-time.Hour)
	repos.Tokens.Create(ctx, &model.Token{ID: "t1", UserID: "u1", Hash: "abc", ExpiresAt: &expired, CreatedAt: time.Now().UTC()})
	repos.Tokens.DeleteExpired(ctx)
	_, err := repos.Tokens.GetByHash(ctx, "abc")
	if err != iface.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete expired, got %v", err)
	}
}

func TestSQLToken_DeleteByUserAndNote(t *testing.T) {
	repos := setupTestDB(t)
	repos.Users.Create(ctx, &model.User{ID: "u1", Username: "alice", PasswordHash: "h", CreatedAt: time.Now().UTC()})
	repos.Tokens.Create(ctx, &model.Token{ID: "t1", UserID: "u1", Hash: "h1", Note: "login", CreatedAt: time.Now().UTC()})
	repos.Tokens.Create(ctx, &model.Token{ID: "t2", UserID: "u1", Hash: "h2", Note: "login", CreatedAt: time.Now().UTC()})
	repos.Tokens.Create(ctx, &model.Token{ID: "t3", UserID: "u1", Hash: "h3", Note: "api", CreatedAt: time.Now().UTC()})
	repos.Tokens.DeleteByUserAndNote(ctx, "u1", "login")
	tokens, _ := repos.Tokens.ListByUser(ctx, "u1")
	if len(tokens) != 1 || tokens[0].Note != "api" {
		t.Fatalf("expected 1 api token remaining, got %d", len(tokens))
	}
}
