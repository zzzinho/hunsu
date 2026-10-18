package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"hunsu/internal/chat"
	"hunsu/internal/llm"
)

func (s *Server) sendChat(w http.ResponseWriter, r *http.Request) {
	var turn chat.Turn
	if err := json.NewDecoder(r.Body).Decode(&turn); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stream := &eventStream{w: w}
	err := s.chat.Send(r.Context(), turn, stream.send)
	switch {
	case err != nil && !stream.started:
		writeError(w, err)
	case err != nil:
		stream.send(llm.Event{Kind: errorKind, Text: err.Error()})
	}
}

const errorKind = "error"

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.chat.Status(r.Context()))
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.chat.Models())
}

func (s *Server) resetChat(w http.ResponseWriter, r *http.Request) {
	if err := s.chat.Reset(); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) undo(w http.ResponseWriter, r *http.Request) {
	if err := s.chat.Undo(r.URL.Query().Get("page")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// 에러를 상태 코드로 돌려줄 수 있도록 첫 이벤트를 보낼 때 응답을 시작한다.
type eventStream struct {
	w       http.ResponseWriter
	started bool
}

func (e *eventStream) send(event llm.Event) {
	if !e.started {
		e.w.Header().Set("Content-Type", "text/event-stream")
		e.w.Header().Set("Cache-Control", "no-store")
		e.started = true
	}
	data, _ := json.Marshal(event)
	fmt.Fprintf(e.w, "data: %s\n\n", data)
	http.NewResponseController(e.w).Flush()
}
