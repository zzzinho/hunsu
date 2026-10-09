package server_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"hunsu/internal/chat"
	"hunsu/internal/claude"
	"hunsu/internal/project/projecttest"
	"hunsu/internal/server"
	"hunsu/internal/ui"
)

type startRunner struct{}

func (startRunner) Run(_ context.Context, _ claude.Request, emit func(claude.Event)) error {
	emit(claude.Event{Kind: claude.Started})
	return nil
}

func newServer(t *testing.T) (http.Handler, string, *ui.Hub) {
	t.Helper()
	p, dir := projecttest.New(t, map[string]string{"page.html": "page.html", "notes.txt": "notes.txt"})
	svc, err := chat.New(p, startRunner{})
	if err != nil {
		t.Fatal(err)
	}
	web := fstest.MapFS{"index.html": {Data: []byte("<title>hunsu</title>")}}
	hub := ui.NewHub()
	return server.New(p, svc, hub, web, 10*time.Millisecond), dir, hub
}

func TestServer_API(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		body       string
		wantStatus int
		wantBody   string
		wantHeader string
	}{
		{"UI 를 준다", "GET", "/", "", 200, "<title>hunsu", ""},
		{"파일 목록을 JSON 으로 준다", "GET", "/api/files", "", 200, `"pages":["page.html"]`, ""},
		{"파일 내용을 준다", "GET", "/api/file?path=page.html", "", 200, "page.html", ""},
		{"파일을 저장한다", "PUT", "/api/file?path=page.html", "saved", 204, "", ""},
		{"템플릿으로 새 파일을 만든다", "POST", "/api/file?path=new.html", "", 201, "", ""},
		{"편집 대상이 아닌 파일은 거부한다", "GET", "/api/file?path=notes.txt", "", 403, "", ""},
		{"없는 파일은 없다고 알린다", "GET", "/api/file?path=none.html", "", 404, "", ""},
		{"이미 있는 파일은 새로 만들지 않는다", "POST", "/api/file?path=page.html", "", 409, "", ""},
		{"렌더용 프로젝트 파일은 캐시 없이 준다", "GET", "/p/page.html", "", 200, "page.html", "Cache-Control: no-store"},
		{"채팅 요청의 진행 이벤트를 스트림으로 보낸다", "POST", "/api/chat", `{"prompt":"고쳐"}`, 200, `data: {"kind":"start"}`, "Content-Type: text/event-stream"},
		{"빈 채팅 요청은 거부한다", "POST", "/api/chat", `{"prompt":""}`, 400, "", ""},
		{"되돌릴 것이 없으면 거부한다", "POST", "/api/undo", "", 409, "", ""},
		{"새 대화를 시작한다", "POST", "/api/chat/reset", "", 204, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _ := newServer(t)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body)))
			key, value, _ := strings.Cut(tt.wantHeader, ": ")
			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantBody) || rec.Header().Get(key) != value {
				t.Errorf("%s %s = %d %q %v; want %d, %q 포함, %q",
					tt.method, tt.target, rec.Code, rec.Body, rec.Header(), tt.wantStatus, tt.wantBody, tt.wantHeader)
			}
		})
	}
}

func TestServer_Events(t *testing.T) {
	tests := []struct {
		name  string
		watch string
		want  string
	}{
		{"감시 중인 파일이 바뀌면 그 경로를 보낸다", "page.html", "data: page.html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, dir, _ := newServer(t)
			srv := httptest.NewServer(h)
			defer srv.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/events?path="+tt.watch, nil)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if err := os.WriteFile(filepath.Join(dir, tt.watch), []byte("changed"), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := firstData(res.Body); got != tt.want {
				t.Errorf("첫 이벤트 = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServer_UIState(t *testing.T) {
	tests := []struct {
		name       string
		origin     string
		wantStatus int
		wantPage   string
	}{
		{"hunsu 화면이 알린 화면 상태를 기억한다", "http://example.com", 204, "a.html"},
		{"다른 웹사이트가 보낸 화면 상태는 거부한다", "http://evil.test", 403, ""},
		{"출처가 없는 요청도 거부한다", "", 403, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, hub := newServer(t)
			req := httptest.NewRequest("POST", "/api/ui", strings.NewReader(`{"page":"a.html"}`))
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus || hub.Current().Page != tt.wantPage {
				t.Errorf("상태 = %d, 기억한 페이지 = %q; want %d, %q", rec.Code, hub.Current().Page, tt.wantStatus, tt.wantPage)
			}
		})
	}
}

func TestServer_UICommands(t *testing.T) {
	tests := []struct {
		name    string
		command ui.Command
		want    string
	}{
		{"서버의 명령을 브라우저로 보낸다", ui.Command{Kind: ui.Open, Page: "a.html"}, `data: {"kind":"open","page":"a.html"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, hub := newServer(t)
			srv := httptest.NewServer(h)
			defer srv.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/ui/commands", nil)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			hub.Send(tt.command)
			if got := firstData(res.Body); got != tt.want {
				t.Errorf("첫 명령 = %q, want %q", got, tt.want)
			}
		})
	}
}

func firstData(body io.Reader) string {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "data:") {
			return line
		}
	}
	return ""
}
