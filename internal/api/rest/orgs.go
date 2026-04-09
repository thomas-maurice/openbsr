package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

type OrgHandler struct {
	repos *iface.Repos
}

func NewOrgHandler(repos *iface.Repos) *OrgHandler {
	return &OrgHandler{repos: repos}
}

func (h *OrgHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{name}", h.getOrg)
	mux.HandleFunc("POST /api/v1/orgs", h.createOrg)
	mux.HandleFunc("POST /api/v1/orgs/{name}/members", h.addMember)
	mux.HandleFunc("DELETE /api/v1/orgs/{name}/members/{username}", h.removeMember)
}

func (h *OrgHandler) getOrg(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	o, err := h.repos.Orgs.GetByName(r.Context(), name)
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "organization not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": o.ID, "name": o.Name})
}

func (h *OrgHandler) createOrg(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !usernameRe.MatchString(req.Name) {
		writeErr(w, http.StatusBadRequest, "org name must be 3-39 chars, lowercase alphanumeric and hyphens")
		return
	}
	o := &model.Org{
		ID:        uuid.NewString(),
		Name:      req.Name,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.repos.Orgs.Create(r.Context(), o); err != nil {
		if _, lookupErr := h.repos.Orgs.GetByName(r.Context(), req.Name); lookupErr == nil {
			writeErr(w, http.StatusConflict, "organization already exists")
			return
		}
		slog.Error("create org", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := h.repos.Orgs.AddMember(r.Context(), o.ID, u.ID, model.OrgRoleAdmin); err != nil {
		slog.Error("add org admin member", "err", err)
		writeErr(w, http.StatusInternalServerError, "failed to add admin member")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": o.ID, "name": o.Name})
}

func (h *OrgHandler) addMember(w http.ResponseWriter, r *http.Request) {
	caller := auth.UserFromContext(r.Context())
	if caller == nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	orgName := r.PathValue("name")
	o, err := h.repos.Orgs.GetByName(r.Context(), orgName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "organization not found")
		return
	}
	if !h.repos.Auth.CanManageOrgMembers(r.Context(), caller.ID, o.ID) {
		writeErr(w, http.StatusForbidden, "must be org admin")
		return
	}
	var req struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	target, err := h.repos.Users.GetByUsername(r.Context(), req.Username)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	role := model.OrgRoleMember
	if req.Role == string(model.OrgRoleAdmin) {
		role = model.OrgRoleAdmin
	}
	if err := h.repos.Orgs.AddMember(r.Context(), o.ID, target.ID, role); err != nil {
		slog.Error("add org member", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (h *OrgHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	caller := auth.UserFromContext(r.Context())
	if caller == nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	orgName := r.PathValue("name")
	o, err := h.repos.Orgs.GetByName(r.Context(), orgName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "organization not found")
		return
	}
	if !h.repos.Auth.CanManageOrgMembers(r.Context(), caller.ID, o.ID) {
		writeErr(w, http.StatusForbidden, "must be org admin")
		return
	}
	username := r.PathValue("username")
	target, err := h.repos.Users.GetByUsername(r.Context(), username)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	if err := h.repos.Orgs.RemoveMember(r.Context(), o.ID, target.ID); err != nil {
		slog.Error("remove org member", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
