package rest

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/thomas-maurice/openbsr/internal/auth"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/storage"
)

type FileHandler struct {
	repos *iface.Repos
	store storage.Store
}

func NewFileHandler(repos *iface.Repos, store storage.Store) *FileHandler {
	return &FileHandler{repos: repos, store: store}
}

func (h *FileHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/modules/{owner}/{repo}/commits/{id}/files", h.listFiles)
	mux.HandleFunc("GET /api/v1/modules/{owner}/{repo}/commits/{id}/file", h.getFileContent)
}

func (h *FileHandler) resolveCommitZip(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	commitID := r.PathValue("id")

	m, err := h.repos.Modules.Get(r.Context(), owner, repo)
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "module not found")
		} else {
			writeErr(w, http.StatusInternalServerError, "internal error")
		}
		return nil, false
	}

	caller := auth.UserFromContext(r.Context())
	uid := ""
	if caller != nil { uid = caller.ID }
	if !h.repos.Auth.CanReadModule(r.Context(), uid, m) {
		writeErr(w, http.StatusNotFound, "module not found")
		return nil, false
	}

	c, err := h.repos.Commits.GetByID(r.Context(), m.ID, commitID)
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "commit not found")
		} else {
			writeErr(w, http.StatusInternalServerError, "internal error")
		}
		return nil, false
	}

	data, err := h.store.Get(r.Context(), c.StorageKey)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read content")
		return nil, false
	}
	return data, true
}

func (h *FileHandler) listFiles(w http.ResponseWriter, r *http.Request) {
	data, ok := h.resolveCommitZip(w, r)
	if !ok {
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read archive")
		return
	}
	var files []map[string]string
	for _, f := range zr.File {
		files = append(files, map[string]string{
			"path": f.Name,
			"size": fmt.Sprintf("%d", f.UncompressedSize64),
		})
	}
	if files == nil {
		files = []map[string]string{}
	}
	writeJSON(w, http.StatusOK, files)
}

func (h *FileHandler) getFileContent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path query parameter required")
		return
	}
	data, ok := h.resolveCommitZip(w, r)
	if !ok {
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read archive")
		return
	}
	for _, f := range zr.File {
		if f.Name == path {
			rc, err := f.Open()
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read file")
				return
			}
			defer rc.Close()
			content, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "failed to read file")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{
				"path":    f.Name,
				"content": base64.StdEncoding.EncodeToString(content),
			})
			return
		}
	}
	writeErr(w, http.StatusNotFound, "file not found")
}
