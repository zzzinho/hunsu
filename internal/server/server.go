package server

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"hunsu/internal/chat"
	"hunsu/internal/project"
	"hunsu/internal/ui"
)

// Renderer 는 디자인 페이지를 PNG 로 찍는다. Chrome 이 없으면 nil 이다.
type Renderer interface {
	Screenshot(ctx context.Context, page string, width, maxHeight int) ([]byte, error)
}

type Server struct {
	project   *project.Project
	chat      *chat.Service
	hub       *ui.Hub
	pollEvery time.Duration
	renderer  Renderer
}

func New(p *project.Project, c *chat.Service, hub *ui.Hub, web fs.FS, pollEvery time.Duration, renderer Renderer) http.Handler {
	s := &Server{project: p, chat: c, hub: hub, pollEvery: pollEvery, renderer: renderer}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/files", s.list)
	mux.HandleFunc("GET /api/file", s.read)
	mux.HandleFunc("PUT /api/file", s.write)
	mux.HandleFunc("POST /api/file", s.create)
	mux.HandleFunc("POST /api/assets", s.addAsset)
	mux.HandleFunc("GET /api/export/zip", s.exportZIP)
	mux.HandleFunc("GET /api/export/html", s.exportHTML)
	mux.HandleFunc("GET /api/export/png", s.exportPNG)
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
