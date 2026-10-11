package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"

	"hunsu/internal/chat"
	"hunsu/internal/export"
	"hunsu/internal/history"
	"hunsu/internal/project"
	"hunsu/internal/render"
)

var errorStatus = []struct {
	err    error
	status int
}{
	{project.ErrNotEditable, http.StatusForbidden},
	{project.ErrNotAsset, http.StatusUnsupportedMediaType},
	{project.ErrTooLarge, http.StatusRequestEntityTooLarge},
	{export.ErrNotPage, http.StatusNotFound},
	{render.ErrWidth, http.StatusBadRequest},
	{errNoRenderer, http.StatusNotImplemented},
	{fs.ErrNotExist, http.StatusNotFound},
	{fs.ErrExist, http.StatusConflict},
	{chat.ErrEmptyPrompt, http.StatusBadRequest},
	{chat.ErrBusy, http.StatusConflict},
	{history.ErrNothingToUndo, http.StatusConflict},
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	listing, err := s.project.List()
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listing)
}

func (s *Server) read(w http.ResponseWriter, r *http.Request) {
	data, err := s.project.Read(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func (s *Server) write(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err == nil {
		err = s.project.Write(r.URL.Query().Get("path"), data)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	if err := s.project.Create(r.URL.Query().Get("path")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) addAsset(w http.ResponseWriter, r *http.Request) {
	if err := s.project.AddAsset(r.URL.Query().Get("name"), r.Body); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	for _, e := range errorStatus {
		if errors.Is(err, e.err) {
			status = e.status
			break
		}
	}
	http.Error(w, err.Error(), status)
}
