package server

import (
	"io/fs"
	"net/http"
	"time"

	"hunsu/internal/chat"
	"hunsu/internal/project"
	"hunsu/internal/ui"
)

type Server struct {
	project   *project.Project
	chat      *chat.Service
	hub       *ui.Hub
	pollEvery time.Duration
}

func New(p *project.Project, c *chat.Service, hub *ui.Hub, web fs.FS, pollEvery time.Duration) http.Handler {
	s := &Server{project: p, chat: c, hub: hub, pollEvery: pollEvery}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/files", s.list)
	mux.HandleFunc("GET /api/file", s.read)
	mux.HandleFunc("PUT /api/file", s.write)
	mux.HandleFunc("POST /api/file", s.create)
	mux.HandleFunc("POST /api/assets", s.addAsset)
	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("POST /api/chat", s.sendChat)
	mux.HandleFunc("POST /api/chat/reset", s.resetChat)
	mux.HandleFunc("GET /api/models", s.models)
	mux.HandleFunc("POST /api/undo", s.undo)
	mux.HandleFunc("POST /api/ui", s.updateUI)
	mux.HandleFunc("GET /api/ui/commands", s.uiCommands)
	mux.Handle("GET /p/", http.StripPrefix("/p/", noStore(http.FileServerFS(p.FS()))))
	mux.Handle("GET /", http.FileServerFS(web))
	return mux
}

func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}
