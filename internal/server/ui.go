package server

import (
	"encoding/json"
	"net/http"

	"hunsu/internal/ui"
)

// 다른 웹사이트가 가짜 선택 요소를 꾸며 넣어 MCP 로 연결된 에이전트에 지시를 끼워 넣지 못하게, hunsu 화면에서 온 요청만 받는다.
func (s *Server) updateUI(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "http://"+r.Host {
		http.Error(w, "only requests from the hunsu page are accepted", http.StatusForbidden)
		return
	}
	var state ui.State
	if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hub.Update(state)
	w.WriteHeader(http.StatusNoContent)
}
