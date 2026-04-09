package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,37}[a-z0-9]$`)

type AuthHandler struct {
	repos   *iface.Repos
	openReg bool
}

func NewAuthHandler(repos *iface.Repos, openReg bool) *AuthHandler {
	return &AuthHandler{repos: repos, openReg: openReg}
}

func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("GET /api/v1/auth/me", h.me)
	mux.HandleFunc("GET /api/v1/auth/tokens", h.listTokens)
	mux.HandleFunc("POST /api/v1/auth/tokens", h.createToken)
	mux.HandleFunc("DELETE /api/v1/auth/tokens/{id}", h.revokeToken)
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	if !h.openReg {
		writeErr(w, http.StatusForbidden, "registration is closed")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !usernameRe.MatchString(req.Username) {
		writeErr(w, http.StatusBadRequest, "username must be 3-39 chars, lowercase alphanumeric and hyphens")
		return
	}
	if len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("hash password", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Check for existing user first (MongoDB has no unique index)
	if _, err := h.repos.Users.GetByUsername(r.Context(), req.Username); err == nil {
		writeErr(w, http.StatusConflict, "username already exists")
		return
	}
	u := &model.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.repos.Users.Create(r.Context(), u); err != nil {
		slog.Error("create user", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": u.ID, "username": u.Username})
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := h.repos.Users.GetByUsername(r.Context(), req.Username)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	// Clean up old login tokens for this user
	h.repos.Tokens.DeleteByUserAndNote(r.Context(), u.ID, "login")

	rawToken := uuid.NewString()
	expires := time.Now().UTC().Add(24 * time.Hour)
	t := &model.Token{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		Hash:      auth.HashToken(rawToken),
		Note:      "login",
		ExpiresAt: &expires,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.repos.Tokens.Create(r.Context(), t); err != nil {
		slog.Error("create token", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": rawToken, "user_id": u.ID, "username": u.Username})
}

func (h *AuthHandler) me(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": u.ID, "username": u.Username})
}

func (h *AuthHandler) listTokens(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	tokens, err := h.repos.Tokens.ListByUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	now := time.Now().UTC()
	var result []map[string]string
	for _, t := range tokens {
		// Skip expired tokens and login session tokens
		if t.ExpiresAt != nil && t.ExpiresAt.Before(now) {
			continue
		}
		if t.Note == "login" {
			continue
		}
		result = append(result, map[string]string{
			"id":   t.ID,
			"note": t.Note,
		})
	}
	if result == nil {
		result = []map[string]string{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AuthHandler) createToken(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	rawToken := uuid.NewString()
	t := &model.Token{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		Hash:      auth.HashToken(rawToken),
		Note:      req.Note,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.repos.Tokens.Create(r.Context(), t); err != nil {
		slog.Error("create token", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": rawToken, "id": t.ID})
}

func (h *AuthHandler) revokeToken(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	id := r.PathValue("id")
	if err := h.repos.Tokens.Revoke(r.Context(), id, u.ID); err != nil {
		writeErr(w, http.StatusNotFound, "token not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
