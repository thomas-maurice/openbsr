package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

// fakeUserRepo for REST handler tests
type fakeUserRepo struct {
	users map[string]*model.User
}

func (f *fakeUserRepo) Create(_ context.Context, u *model.User) error {
	for _, existing := range f.users {
		if existing.Username == u.Username {
			return iface.ErrNotFound // simulate duplicate
		}
	}
	f.users[u.ID] = u
	return nil
}
func (f *fakeUserRepo) GetByUsername(_ context.Context, username string) (*model.User, error) {
	for _, u := range f.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, iface.ErrNotFound
}
func (f *fakeUserRepo) GetByID(_ context.Context, id string) (*model.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, iface.ErrNotFound
	}
	return u, nil
}
func (f *fakeUserRepo) ListByOrg(_ context.Context, orgID string) ([]*model.User, error) {
	return nil, nil
}

type fakeTokenRepo struct {
	tokens map[string]*model.Token
}

func (f *fakeTokenRepo) Create(_ context.Context, t *model.Token) error {
	f.tokens[t.Hash] = t
	return nil
}
func (f *fakeTokenRepo) GetByHash(_ context.Context, hash string) (*model.Token, error) {
	t, ok := f.tokens[hash]
	if !ok {
		return nil, iface.ErrNotFound
	}
	return t, nil
}
func (f *fakeTokenRepo) ListByUser(_ context.Context, userID string) ([]*model.Token, error) {
	return nil, nil
}
func (f *fakeTokenRepo) Revoke(_ context.Context, id, userID string) error              { return nil }
func (f *fakeTokenRepo) DeleteExpired(_ context.Context) error                         { return nil }
func (f *fakeTokenRepo) DeleteByUserAndNote(_ context.Context, userID, note string) error { return nil }

func setupHandler() (*AuthHandler, *http.ServeMux) {
	repos := &iface.Repos{
		Users:  &fakeUserRepo{users: make(map[string]*model.User)},
		Tokens: &fakeTokenRepo{tokens: make(map[string]*model.Token)},
	}
	h := NewAuthHandler(repos, true)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux
}

func TestRegister_Success(t *testing.T) {
	_, mux := setupHandler()
	body := `{"username":"testuser","password":"longenough"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["username"] != "testuser" {
		t.Fatalf("expected username testuser, got %s", resp["username"])
	}
}

func TestRegister_BadUsername(t *testing.T) {
	_, mux := setupHandler()
	body := `{"username":"AB","password":"longenough"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	_, mux := setupHandler()
	body := `{"username":"testuser","password":"short"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegister_ClosedRegistration(t *testing.T) {
	repos := &iface.Repos{
		Users:  &fakeUserRepo{users: make(map[string]*model.User)},
		Tokens: &fakeTokenRepo{tokens: make(map[string]*model.Token)},
	}
	h := NewAuthHandler(repos, false) // registration closed
	mux := http.NewServeMux()
	h.Register(mux)
	body := `{"username":"testuser","password":"longenough"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestLogin_Success(t *testing.T) {
	_, mux := setupHandler()

	// Register first
	body := `{"username":"testuser","password":"longenough"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", rec.Code, rec.Body.String())
	}

	// Login
	body = `{"username":"testuser","password":"longenough"}`
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Fatal("expected token in response")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	_, mux := setupHandler()
	// Register
	body := `{"username":"testuser","password":"longenough"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Login with wrong password
	body = `{"username":"testuser","password":"wrongpass"}`
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	_, mux := setupHandler()
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMe_Authenticated(t *testing.T) {
	_, mux := setupHandler()
	user := &model.User{ID: "u1", Username: "alice", CreatedAt: time.Now()}
	ctx := auth.ContextWithUser(context.Background(), user)
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["username"] != "alice" {
		t.Fatalf("expected alice, got %s", resp["username"])
	}
}
