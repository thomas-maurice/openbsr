package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/model"
)

func TestCreateOrg_Success(t *testing.T) {
	mux, repos := setupM2Handler(t)
	orgH := NewOrgHandler(repos)
	orgH.Register(mux)

	// Register a user first
	u := createTestUser(t, repos, "orgcreator")

	body, _ := json.Marshal(map[string]string{"name": "test-org"})
	req := httptest.NewRequest("POST", "/api/v1/orgs", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "test-org" {
		t.Fatalf("expected name test-org, got %s", resp["name"])
	}
}

func TestCreateOrg_Unauthenticated(t *testing.T) {
	mux, repos := setupM2Handler(t)
	orgH := NewOrgHandler(repos)
	orgH.Register(mux)

	body, _ := json.Marshal(map[string]string{"name": "test-org"})
	req := httptest.NewRequest("POST", "/api/v1/orgs", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetOrg_Success(t *testing.T) {
	mux, repos := setupM2Handler(t)
	orgH := NewOrgHandler(repos)
	orgH.Register(mux)

	repos.Orgs.Create(nil, &model.Org{ID: "org1", Name: "my-org"})

	req := httptest.NewRequest("GET", "/api/v1/orgs/my-org", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetOrg_NotFound(t *testing.T) {
	mux, repos := setupM2Handler(t)
	orgH := NewOrgHandler(repos)
	orgH.Register(mux)

	req := httptest.NewRequest("GET", "/api/v1/orgs/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAddMember_Success(t *testing.T) {
	mux, repos := setupM2Handler(t)
	orgH := NewOrgHandler(repos)
	orgH.Register(mux)

	admin := createTestUser(t, repos, "orgadmin")
	member := createTestUser(t, repos, "newmember")
	repos.Orgs.Create(nil, &model.Org{ID: "org1", Name: "my-org"})
	repos.Orgs.AddMember(nil, "org1", admin.ID, model.OrgRoleAdmin)

	body, _ := json.Marshal(map[string]string{"username": member.Username, "role": "member"})
	req := httptest.NewRequest("POST", "/api/v1/orgs/my-org/members", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddMember_NotAdmin(t *testing.T) {
	mux, repos := setupM2Handler(t)
	orgH := NewOrgHandler(repos)
	orgH.Register(mux)

	nonAdmin := createTestUser(t, repos, "regularuser")
	member := createTestUser(t, repos, "newmember2")
	repos.Orgs.Create(nil, &model.Org{ID: "org2", Name: "other-org"})
	repos.Orgs.AddMember(nil, "org2", nonAdmin.ID, model.OrgRoleMember)

	body, _ := json.Marshal(map[string]string{"username": member.Username, "role": "member"})
	req := httptest.NewRequest("POST", "/api/v1/orgs/other-org/members", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), nonAdmin))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRemoveMember_Success(t *testing.T) {
	mux, repos := setupM2Handler(t)
	orgH := NewOrgHandler(repos)
	orgH.Register(mux)

	admin := createTestUser(t, repos, "rmadmin")
	member := createTestUser(t, repos, "rmmember")
	repos.Orgs.Create(nil, &model.Org{ID: "org3", Name: "rm-org"})
	repos.Orgs.AddMember(nil, "org3", admin.ID, model.OrgRoleAdmin)
	repos.Orgs.AddMember(nil, "org3", member.ID, model.OrgRoleMember)

	req := httptest.NewRequest("DELETE", "/api/v1/orgs/rm-org/members/"+member.Username, nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
