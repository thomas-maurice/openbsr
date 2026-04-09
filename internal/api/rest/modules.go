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

type ModuleHandler struct {
	repos *iface.Repos
}

func NewModuleHandler(repos *iface.Repos) *ModuleHandler {
	return &ModuleHandler{repos: repos}
}

func (h *ModuleHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/modules/{owner}/{repo}", h.getModule)
	mux.HandleFunc("GET /api/v1/modules", h.listModules)
	mux.HandleFunc("POST /api/v1/modules", h.createModule)
}

func (h *ModuleHandler) getModule(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	m, err := h.repos.Modules.Get(r.Context(), owner, repo)
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "module not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	caller := auth.UserFromContext(r.Context())
	uid := ""
	if caller != nil { uid = caller.ID }
	if !h.repos.Auth.CanReadModule(r.Context(), uid, m) {
		writeErr(w, http.StatusNotFound, "module not found")
		return
	}
	writeJSON(w, http.StatusOK, moduleToJSON(m))
}

func (h *ModuleHandler) listModules(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	query := r.URL.Query().Get("q")

	var mods []*model.Module
	var err error

	if owner != "" {
		// List by owner (existing behavior)
		mods, err = h.repos.Modules.List(r.Context(), owner)
	} else {
		// Search/list public modules
		mods, err = h.repos.Modules.ListPublic(r.Context(), query, 50)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	caller := auth.UserFromContext(r.Context())
	uid := ""
	if caller != nil { uid = caller.ID }
	var result []map[string]string
	for _, m := range mods {
		if !h.repos.Auth.CanReadModule(r.Context(), uid, m) {
			continue
		}
		result = append(result, moduleToJSON(m))
	}
	if result == nil {
		result = []map[string]string{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *ModuleHandler) createModule(w http.ResponseWriter, r *http.Request) {
	caller := auth.UserFromContext(r.Context())
	if caller == nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	var req struct {
		Owner      string `json:"owner"`
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !usernameRe.MatchString(req.Name) {
		writeErr(w, http.StatusBadRequest, "module name must be 2-39 chars, lowercase alphanumeric and hyphens")
		return
	}
	if !h.repos.Auth.CanCreateModule(r.Context(), caller.ID, req.Owner) {
		writeErr(w, http.StatusForbidden, "no permission to create modules under this owner")
		return
	}
	var ownerType model.OwnerType
	if req.Owner == caller.Username {
		ownerType = model.OwnerTypeUser
	} else {
		ownerType = model.OwnerTypeOrg
	}
	vis := model.VisibilityPrivate
	if req.Visibility == "public" {
		vis = model.VisibilityPublic
	}
	// Check for existing module first (MongoDB has no unique index)
	if _, err := h.repos.Modules.Get(r.Context(), req.Owner, req.Name); err == nil {
		writeErr(w, http.StatusConflict, "module already exists")
		return
	}
	m := &model.Module{
		ID:         uuid.NewString(),
		OwnerName:  req.Owner,
		OwnerType:  ownerType,
		Name:       req.Name,
		Visibility: vis,
		CreatedAt:  time.Now().UTC(),
	}
	if err := h.repos.Modules.Create(r.Context(), m); err != nil {
		slog.Error("create module", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, moduleToJSON(m))
}


func moduleToJSON(m *model.Module) map[string]string {
	return map[string]string{
		"id":         m.ID,
		"owner":      m.OwnerName,
		"name":       m.Name,
		"visibility": string(m.Visibility),
	}
}
