package rest

import (
	"errors"
	"net/http"

	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

type CommitHandler struct {
	repos *iface.Repos
}

func NewCommitHandler(repos *iface.Repos) *CommitHandler {
	return &CommitHandler{repos: repos}
}

func (h *CommitHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/modules/{owner}/{repo}/commits", h.listCommits)
	mux.HandleFunc("GET /api/v1/modules/{owner}/{repo}/commits/{id}", h.getCommit)
	mux.HandleFunc("GET /api/v1/modules/{owner}/{repo}/labels", h.listLabels)
}

// checkModuleAccess resolves the module and checks visibility. Returns nil module on error (already written to w).
func (h *CommitHandler) checkModuleAccess(w http.ResponseWriter, r *http.Request) *model.Module {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	m, err := h.repos.Modules.Get(r.Context(), owner, repo)
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "module not found")
		} else {
			writeErr(w, http.StatusInternalServerError, "internal error")
		}
		return nil
	}
	caller := auth.UserFromContext(r.Context())
	uid := ""
	if caller != nil { uid = caller.ID }
	if !h.repos.Auth.CanReadModule(r.Context(), uid, m) {
		writeErr(w, http.StatusNotFound, "module not found")
		return nil
	}
	return m
}

func (h *CommitHandler) listCommits(w http.ResponseWriter, r *http.Request) {
	m := h.checkModuleAccess(w, r)
	if m == nil {
		return
	}
	commits, err := h.repos.Commits.ListByModule(r.Context(), m.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	var result []map[string]string
	for _, c := range commits {
		result = append(result, map[string]string{
			"id":         c.ID,
			"module_id":  c.ModuleID,
			"created_at": c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	if result == nil {
		result = []map[string]string{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *CommitHandler) getCommit(w http.ResponseWriter, r *http.Request) {
	m := h.checkModuleAccess(w, r)
	if m == nil {
		return
	}
	commitID := r.PathValue("id")
	c, err := h.repos.Commits.GetByID(r.Context(), m.ID, commitID)
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "commit not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"id":         c.ID,
		"module_id":  c.ModuleID,
		"created_at": c.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *CommitHandler) listLabels(w http.ResponseWriter, r *http.Request) {
	m := h.checkModuleAccess(w, r)
	if m == nil {
		return
	}
	labels, err := h.repos.Labels.List(r.Context(), m.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	var result []map[string]string
	for _, l := range labels {
		result = append(result, map[string]string{
			"name":      l.Name,
			"commit_id": l.CommitID,
		})
	}
	if result == nil {
		result = []map[string]string{}
	}
	writeJSON(w, http.StatusOK, result)
}
