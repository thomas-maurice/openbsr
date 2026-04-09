package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

// fakeTokenRepo implements iface.TokenRepo for testing
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

// fakeUserRepo implements iface.UserRepo for testing
type fakeUserRepo struct {
	users map[string]*model.User
}

func (f *fakeUserRepo) Create(_ context.Context, u *model.User) error {
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

func TestMiddleware_NoToken(t *testing.T) {
	repos := &iface.Repos{
		Tokens: &fakeTokenRepo{tokens: make(map[string]*model.Token)},
		Users:  &fakeUserRepo{users: make(map[string]*model.User)},
	}
	handler := Middleware(repos)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u != nil {
			t.Fatal("user should be nil when no token provided")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	rawToken := "test-token-123"
	hash := HashToken(rawToken)
	user := &model.User{ID: "user-1", Username: "alice"}

	repos := &iface.Repos{
		Tokens: &fakeTokenRepo{tokens: map[string]*model.Token{
			hash: {ID: "tok-1", UserID: "user-1", Hash: hash, CreatedAt: time.Now()},
		}},
		Users: &fakeUserRepo{users: map[string]*model.User{
			"user-1": user,
		}},
	}

	handler := Middleware(repos)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil {
			t.Fatal("user should be set for valid token")
		}
		if u.Username != "alice" {
			t.Fatalf("expected alice, got %s", u.Username)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	repos := &iface.Repos{
		Tokens: &fakeTokenRepo{tokens: make(map[string]*model.Token)},
		Users:  &fakeUserRepo{users: make(map[string]*model.User)},
	}
	handler := Middleware(repos)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u != nil {
			t.Fatal("user should be nil for invalid token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	rawToken := "expired-token"
	hash := HashToken(rawToken)
	expired := time.Now().Add(-1 * time.Hour)
	user := &model.User{ID: "user-1", Username: "alice"}

	repos := &iface.Repos{
		Tokens: &fakeTokenRepo{tokens: map[string]*model.Token{
			hash: {ID: "tok-1", UserID: "user-1", Hash: hash, ExpiresAt: &expired, CreatedAt: time.Now()},
		}},
		Users: &fakeUserRepo{users: map[string]*model.User{
			"user-1": user,
		}},
	}

	handler := Middleware(repos)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u != nil {
			t.Fatal("user should be nil for expired token")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
