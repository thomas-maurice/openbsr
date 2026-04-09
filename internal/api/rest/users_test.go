package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUser_Success(t *testing.T) {
	mux, repos := setupM2Handler(t)
	userH := NewUserHandler(repos)
	userH.Register(mux)

	createTestUser(t, repos, "lookupuser")

	req := httptest.NewRequest("GET", "/api/v1/users/lookupuser", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["username"] != "lookupuser" {
		t.Fatalf("expected username lookupuser, got %s", resp["username"])
	}
}

func TestGetUser_NotFound(t *testing.T) {
	mux, repos := setupM2Handler(t)
	userH := NewUserHandler(repos)
	userH.Register(mux)

	req := httptest.NewRequest("GET", "/api/v1/users/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
