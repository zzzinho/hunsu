package server

import (
	"fmt"
	"net/http"
	"time"

	"hunsu/internal/watch"
)

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	poller := watch.NewPoller(s.project.FS(), r.URL.Query()["path"])
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	flusher := http.NewResponseController(w)
	if err := flusher.Flush(); err != nil {
		return
	}
	ticker := time.NewTicker(s.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			changed := poller.Poll()
			for _, name := range changed {
				fmt.Fprintf(w, "data: %s\n\n", name)
			}
			if len(changed) > 0 && flusher.Flush() != nil {
				return
			}
		}
	}
}
