package server

import (
	"context"
	"io"
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
	mux.HandleFunc("POST /api/chat/variants", s.sendVariants)
	mux.HandleFunc("GET /api/variants", s.variants)
	mux.HandleFunc("POST /api/variants/choose", s.chooseVariant)
	mux.HandleFunc("POST /api/variants/discard", s.discardVariants)
	mux.HandleFunc("GET /variant/{id}/{path...}", s.variantFile)
	mux.HandleFunc("GET /api/models", s.models)
	mux.HandleFunc("POST /api/undo", s.undo)
	mux.HandleFunc("GET /api/versions", s.versions)
	mux.HandleFunc("POST /api/versions", s.saveVersion)
	mux.HandleFunc("POST /api/versions/restore", s.restoreVersion)
	mux.HandleFunc("GET /api/versions/thumb", s.versionThumb)
	mux.HandleFunc("POST /api/ui", s.updateUI)
	mux.HandleFunc("GET /api/ui/commands", s.uiCommands)
	mux.Handle("GET /p/", http.StripPrefix("/p/", designFiles(p.FS())))
	mux.Handle("GET /", http.FileServerFS(web))
	return mux
}

// 렌더에는 디자인 파일만 준다. .hunsu/ 와 폴더 목록은 주지 않는다.
// http.FileServer 는 index.html 을 폴더 주소로 돌려보내므로 파일을 직접 연다.
func designFiles(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path
		if !project.IsDesignFile(name) {
			http.NotFound(w, r)
			return
		}
		f, err := fsys.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		content, seekable := f.(io.ReadSeeker)
		if err != nil || info.IsDir() || !seekable {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, name, info.ModTime(), content)
	})
}
