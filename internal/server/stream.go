package server

import (
	"fmt"
	"net/http"
)

type eventWriter struct {
	w          http.ResponseWriter
	controller *http.ResponseController
}

func startEvents(w http.ResponseWriter) (*eventWriter, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	e := &eventWriter{w: w, controller: http.NewResponseController(w)}
	return e, e.controller.Flush()
}

func (e *eventWriter) send(data string) error {
	fmt.Fprintf(e.w, "data: %s\n\n", data)
	return e.controller.Flush()
}
