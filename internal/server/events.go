package server

import (
	"net/http"
	"time"

	"hunsu/internal/watch"
)

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	poller := watch.NewPoller(s.project.FS(), r.URL.Query()["path"])
	stream, err := startEvents(w)
	if err != nil {
		return
	}
	ticker := time.NewTicker(s.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			for _, name := range poller.Poll() {
				if stream.send(name) != nil {
					return
				}
			}
		}
	}
}
