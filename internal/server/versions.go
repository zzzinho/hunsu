package server

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) versions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.chat.Versions()
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}

func (s *Server) saveVersion(w http.ResponseWriter, r *http.Request) {
	saved, err := s.chat.SaveVersion(r.URL.Query().Get("page"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"saved": saved})
}

func (s *Server) restoreVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "버전 번호가 숫자가 아니다", http.StatusBadRequest)
		return
	}
	if err := s.chat.Restore(id, r.URL.Query().Get("page")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) versionThumb(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "버전 번호가 숫자가 아니다", http.StatusBadRequest)
		return
	}
	png, err := s.chat.Thumbnail(id)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(png)
}
