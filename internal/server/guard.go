package server

import (
	"net"
	"net/http"
)

// Guard 는 로컬 서버를 다른 웹사이트로부터 지킨다. 서버는 127.0.0.1 만 받지만, 사용자가 연 웹사이트는 브라우저를 통해 닿는다.
// 호스트 이름이 다르면 DNS rebinding 이고, 출처가 다르면 CSRF 다. 출처가 없는 요청은 브라우저가 아닌 로컬 프로그램(MCP 클라이언트, curl)이다.
func Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "unknown host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			http.Error(w, "cross-origin request", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
