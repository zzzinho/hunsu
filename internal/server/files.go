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

var errBadID = errors.New("the id must be a number")

// code 는 화면이 오류를 사용자의 언어로 보여 줄 때 쓴다.
var errorStatus = []struct {
	err    error
	status int
	code   string
}{
	{project.ErrNotEditable, http.StatusForbidden, "not-editable"},
	{project.ErrNotAsset, http.StatusUnsupportedMediaType, "not-asset"},
	{project.ErrTooLarge, http.StatusRequestEntityTooLarge, "too-large"},
	{export.ErrNotPage, http.StatusNotFound, "not-page"},
	{render.ErrWidth, http.StatusBadRequest, "width"},
	{errNoRenderer, http.StatusNotImplemented, "no-chrome"},
	{errBadID, http.StatusBadRequest, "bad-id"},
	{fs.ErrNotExist, http.StatusNotFound, "not-found"},
	{fs.ErrExist, http.StatusConflict, "exists"},
	{chat.ErrEmptyPrompt, http.StatusBadRequest, "empty-prompt"},
	{chat.ErrBusy, http.StatusConflict, "busy"},
	{history.ErrNothingToUndo, http.StatusConflict, "nothing-to-undo"},
	{history.ErrNoVersion, http.StatusNotFound, "no-version"},
	{history.ErrNoVariant, http.StatusNotFound, "no-variant"},
	{chat.ErrVariantCount, http.StatusBadRequest, "variant-count"},
	{chat.ErrNoVariants, http.StatusConflict, "no-variants"},
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
	status, code := http.StatusInternalServerError, "internal"
	for _, e := range errorStatus {
		if errors.Is(err, e.err) {
			status, code = e.status, e.code
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"code": code, "message": err.Error()})
}
