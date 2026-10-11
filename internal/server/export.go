package server

import (
	"bytes"
	"context"
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"hunsu/internal/export"
	"hunsu/internal/render"
)

const (
	// Chrome 이 한 번에 찍을 수 있는 높이에 맞춘다.
	exportHeight  = 16000
	exportTimeout = time.Minute
)

var errNoRenderer = errors.New("PNG export needs Chrome. Install it and restart hunsu")

func (s *Server) exportZIP(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := export.ZIP(&buf, s.project); err != nil {
		writeError(w, err)
		return
	}
	attach(w, "application/zip", filepath.Base(s.project.Dir())+".zip", buf.Bytes())
}

func (s *Server) exportHTML(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	data, err := export.HTML(s.project, page)
	if err != nil {
		writeError(w, err)
		return
	}
	attach(w, "text/html; charset=utf-8", page, data)
}

func (s *Server) exportPNG(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	requested, _ := strconv.Atoi(r.URL.Query().Get("width"))
	png, err := s.screenshot(r.Context(), page, requested)
	if err != nil {
		writeError(w, err)
		return
	}
	attach(w, "image/png", strings.TrimSuffix(page, ".html")+".png", png)
}

func (s *Server) screenshot(ctx context.Context, page string, requested int) ([]byte, error) {
	if s.renderer == nil {
		return nil, errNoRenderer
	}
	width, err := render.Width(requested)
	if err != nil {
		return nil, err
	}
	listing, err := s.project.List()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(listing.Pages, page) {
		return nil, export.ErrNotPage
	}
	ctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()
	return s.renderer.Screenshot(ctx, page, width, exportHeight)
}

func attach(w http.ResponseWriter, contentType, filename string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Write(data)
}
