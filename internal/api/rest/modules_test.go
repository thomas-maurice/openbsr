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

func TestCreateModule_Success(t *testing.T) {
	mux, repos := setupM2Handler(t)
	modH := NewModuleHandler(repos)
	modH.Register(mux)

	u := createTestUser(t, repos, "modowner")

	body, _ := json.Marshal(map[string]string{"owner": "modowner", "name": "mymodule", "visibility": "public"})
	req := httptest.NewRequest("POST", "/api/v1/modules", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "mymodule" {
		t.Fatalf("expected name mymodule, got %s", resp["name"])
	}
	if resp["visibility"] != "public" {
		t.Fatalf("expected visibility public, got %s", resp["visibility"])
	}
}

func TestCreateModule_OrgOwner(t *testing.T) {
	mux, repos := setupM2Handler(t)
	modH := NewModuleHandler(repos)
	modH.Register(mux)

	u := createTestUser(t, repos, "orgmodowner")
	repos.Orgs.Create(nil, &model.Org{ID: "org1", Name: "myorg"})
	repos.Orgs.AddMember(nil, "org1", u.ID, model.OrgRoleAdmin)

	body, _ := json.Marshal(map[string]string{"owner": "myorg", "name": "orgmod", "visibility": "private"})
	req := httptest.NewRequest("POST", "/api/v1/modules", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateModule_Unauthenticated(t *testing.T) {
	mux, repos := setupM2Handler(t)
	modH := NewModuleHandler(repos)
	modH.Register(mux)

	body, _ := json.Marshal(map[string]string{"owner": "someone", "name": "mod", "visibility": "public"})
	req := httptest.NewRequest("POST", "/api/v1/modules", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateModule_NotOrgMember(t *testing.T) {
	mux, repos := setupM2Handler(t)
	modH := NewModuleHandler(repos)
	modH.Register(mux)

	u := createTestUser(t, repos, "outsider")
	repos.Orgs.Create(nil, &model.Org{ID: "org2", Name: "privateorg"})

	body, _ := json.Marshal(map[string]string{"owner": "privateorg", "name": "sneaky", "visibility": "public"})
	req := httptest.NewRequest("POST", "/api/v1/modules", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetModule_Public(t *testing.T) {
	mux, repos := setupM2Handler(t)
	modH := NewModuleHandler(repos)
	modH.Register(mux)

	repos.Modules.Create(nil, &model.Module{ID: "m1", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "pubmod", Visibility: model.VisibilityPublic})

	req := httptest.NewRequest("GET", "/api/v1/modules/alice/pubmod", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetModule_PrivateUnauthorized(t *testing.T) {
	mux, repos := setupM2Handler(t)
	modH := NewModuleHandler(repos)
	modH.Register(mux)

	repos.Modules.Create(nil, &model.Module{ID: "m2", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "privmod", Visibility: model.VisibilityPrivate})

	req := httptest.NewRequest("GET", "/api/v1/modules/alice/privmod", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for private module, got %d", w.Code)
	}
}

func TestGetModule_PrivateOwner(t *testing.T) {
	mux, repos := setupM2Handler(t)
	modH := NewModuleHandler(repos)
	modH.Register(mux)

	u := createTestUser(t, repos, "alice")
	repos.Modules.Create(nil, &model.Module{ID: "m3", OwnerName: "alice", OwnerType: model.OwnerTypeUser, Name: "privmod2", Visibility: model.VisibilityPrivate})

	req := httptest.NewRequest("GET", "/api/v1/modules/alice/privmod2", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for owner, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListModules_FiltersPrivate(t *testing.T) {
	mux, repos := setupM2Handler(t)
	modH := NewModuleHandler(repos)
	modH.Register(mux)

	repos.Modules.Create(nil, &model.Module{ID: "m4", OwnerName: "bob", OwnerType: model.OwnerTypeUser, Name: "pub1", Visibility: model.VisibilityPublic})
	repos.Modules.Create(nil, &model.Module{ID: "m5", OwnerName: "bob", OwnerType: model.OwnerTypeUser, Name: "priv1", Visibility: model.VisibilityPrivate})

	req := httptest.NewRequest("GET", "/api/v1/modules?owner=bob", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 public module, got %d", len(resp))
	}
	if resp[0]["name"] != "pub1" {
		t.Fatalf("expected pub1, got %s", resp[0]["name"])
	}
}
