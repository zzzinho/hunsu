package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"

	"hunsu/internal/chat"
	"hunsu/internal/llm"
)

func (s *Server) sendVariants(w http.ResponseWriter, r *http.Request) {
	var in struct {
		chat.Turn
		Count int `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stream := &eventStream{w: w}
	err := s.chat.SendVariants(r.Context(), in.Turn, in.Count, stream.send)
	switch {
	case err != nil && !stream.started:
		writeError(w, err)
	case err != nil:
		stream.send(llm.Event{Kind: errorKind, Text: err.Error()})
	}
}

func (s *Server) variants(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.chat.Variants())
}

func (s *Server) chooseVariant(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		writeError(w, errBadID)
		return
	}
	if err := s.chat.ChooseVariant(id, r.URL.Query().Get("page")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) discardVariants(w http.ResponseWriter, r *http.Request) {
	if err := s.chat.DiscardVariants(); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// 안에는 페이지와 디자인 시스템만 있으므로, 없는 파일(에셋)은 디자인 폴더에서 준다.
func (s *Server) variantFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	files, err := s.chat.VariantFiles(id)
	if err != nil {
		writeError(w, err)
		return
	}
	prefix := "/variant/" + r.PathValue("id") + "/"
	http.StripPrefix(prefix, designFiles(overlay{top: files, base: s.project.FS()})).ServeHTTP(w, r)
}

type overlay struct {
	top, base fs.FS
}

func (o overlay) Open(name string) (fs.File, error) {
	f, err := o.top.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return o.base.Open(name)
	}
	return f, err
}
