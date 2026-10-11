package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"hunsu/internal/server"
)

func TestGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	tests := []struct {
		name   string
		method string
		host   string
		origin string
		want   int
	}{
		{"127.0.0.1 로 온 요청을 받는다", "GET", "127.0.0.1:7777", "", 200},
		{"localhost 로 온 요청을 받는다", "GET", "localhost:7777", "", 200},
		{"다른 호스트 이름으로 온 요청은 거부한다 (DNS rebinding)", "GET", "evil.test:7777", "", 403},
		{"hunsu 화면에서 보낸 요청을 받는다", "POST", "127.0.0.1:7777", "http://127.0.0.1:7777", 200},
		{"다른 웹사이트에서 보낸 요청은 거부한다 (CSRF)", "POST", "127.0.0.1:7777", "http://evil.test", 403},
		{"다른 웹사이트는 읽기 요청도 거부한다", "GET", "127.0.0.1:7777", "http://evil.test", 403},
		{"출처를 감춘 요청(null)은 거부한다", "POST", "127.0.0.1:7777", "null", 403},
		{"출처가 없는 요청은 브라우저가 아닌 로컬 프로그램으로 보고 받는다", "POST", "127.0.0.1:7777", "", 200},
	}
	// 받은 요청의 응답은 다른 웹사이트의 iframe 에 넣을 수 없다 (clickjacking).
	const frameAncestors = "frame-ancestors 'self'"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/undo", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			server.Guard(ok).ServeHTTP(rec, req)
			csp := rec.Header().Get("Content-Security-Policy")
			if rec.Code != tt.want || (rec.Code == 200 && csp != frameAncestors) {
				t.Errorf("%s Host=%s Origin=%q = %d (CSP %q), want %d (CSP %q)", tt.method, tt.host, tt.origin, rec.Code, csp, tt.want, frameAncestors)
			}
		})
	}
}
