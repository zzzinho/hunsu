package server

import (
	"encoding/json"
	"net/http"
	"time"

	"hunsu/internal/watch"
)

// 탭 하나가 연결 하나만 쓰도록 파일 변경(change)과 화면 명령(command)을 한 스트림으로 보낸다.
// 브라우저는 한 호스트에 HTTP/1.1 연결을 6개까지만 연다.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	poller := watch.NewPoller(s.project.FS(), r.URL.Query()["path"])
	commands, cancel := s.hub.Subscribe()
	defer cancel()
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
		case command := <-commands:
			data, _ := json.Marshal(command)
			if stream.send("command", string(data)) != nil {
				return
			}
		case <-ticker.C:
			for _, name := range poller.Poll() {
				if stream.send("change", name) != nil {
					return
				}
			}
		}
	}
}
