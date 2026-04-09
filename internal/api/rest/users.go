package rest

import (
	"errors"
	"net/http"

	"github.com/thomas-maurice/openbsr/internal/db/iface"
)

type UserHandler struct {
	repos *iface.Repos
}

func NewUserHandler(repos *iface.Repos) *UserHandler {
	return &UserHandler{repos: repos}
}

func (h *UserHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users/{username}", h.getUser)
}

func (h *UserHandler) getUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	u, err := h.repos.Users.GetByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": u.ID, "username": u.Username})
}
