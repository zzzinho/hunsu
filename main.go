package main

import (
	"cmp"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"hunsu/internal/chat"
	"hunsu/internal/claude"
	"hunsu/internal/llm"
	"hunsu/internal/mcp"
	"hunsu/internal/mcptools"
	"hunsu/internal/project"
	"hunsu/internal/server"
	"hunsu/internal/ui"
)

const version = "0.1.0"

//go:embed web
var webFiles embed.FS

func main() {
	noOpen := flag.Bool("no-open", false, "브라우저를 자동으로 열지 않는다")
	// MCP 클라이언트에 등록한 주소가 실행할 때마다 바뀌지 않도록 포트를 고정한다.
	port := flag.Int("port", 7777, "서버 포트")
	flag.Parse()
	if err := run(cmp.Or(flag.Arg(0), "."), *port, !*noOpen); err != nil {
		fmt.Fprintln(os.Stderr, "hunsu:", err)
		os.Exit(1)
	}
}

func run(dir string, port int, openBrowser bool) error {
	p, err := project.Open(dir)
	if err != nil {
		return err
	}
	defer p.Close()
	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		return err
	}
	assistant, err := chat.New(p, provider())
	if err != nil {
		return err
	}
	hub := ui.NewHub()
	token, err := mcpToken()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("%w (다른 포트는 -port 로 지정한다)", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireToken(token, mcp.NewServer(mcp.Info{Name: "hunsu", Version: version}, mcptools.Instructions(), mcptools.New(p, assistant, hub)...)))
	mux.Handle("/", server.New(p, assistant, hub, web, 500*time.Millisecond))

	url := "http://" + ln.Addr().String() + "/"
	fmt.Printf("hunsu: %s 를 %s 에서 엽니다\n", p.Dir(), url)
	fmt.Printf("MCP 서버: %smcp (헤더 Authorization: Bearer %s). 클라이언트별 등록 방법은 README 참고\n", url, token)
	if openBrowser {
		exec.Command("open", url).Start()
	}
	return http.Serve(ln, mux)
}

// 브라우저 채팅을 처리할 LLM 을 고르는 유일한 곳이다. PoC 는 Claude Code CLI 를 쓰고,
// 다른 LLM 은 llm.Provider 를 구현한 뒤 여기서 바꾼다.
func provider() llm.Provider {
	return claude.CLI{Path: "claude"}
}

func mcpToken() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return mcp.LoadToken(filepath.Join(configDir, "hunsu", "mcp-token"))
}
