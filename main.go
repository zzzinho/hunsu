package main

import (
	"cmp"
	"context"
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
	"hunsu/internal/codex"
	"hunsu/internal/history"
	"hunsu/internal/llm"
	"hunsu/internal/mcp"
	"hunsu/internal/mcptools"
	"hunsu/internal/project"
	"hunsu/internal/render"
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
	providerName := flag.String("provider", "", "브라우저 채팅의 LLM: claude, codex. 생략하면 설치된 것을 claude → codex 순으로 찾는다")
	flag.Parse()
	if err := run(cmp.Or(flag.Arg(0), "."), *port, !*noOpen, *providerName); err != nil {
		fmt.Fprintln(os.Stderr, "hunsu:", err)
		os.Exit(1)
	}
}

func run(dir string, port int, openBrowser bool, providerName string) error {
	p, err := project.Open(dir)
	if err != nil {
		return err
	}
	defer p.Close()
	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		return err
	}
	token, err := mcpToken()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("%w (다른 포트는 -port 로 지정한다)", err)
	}
	url := "http://" + ln.Addr().String() + "/"
	renderer := findRenderer(url + "p/")
	var tools *llm.MCPServer
	if renderer != nil {
		tools = &llm.MCPServer{Name: "hunsu", URL: url + "mcp/render", Token: token}
	}
	llmProvider, err := provider(providerName, tools)
	if err != nil {
		return err
	}
	assistant, err := chat.New(p, llmProvider, thumbnailer(renderer))
	if err != nil {
		return err
	}
	hub := ui.NewHub()

	info := mcp.Info{Name: "hunsu", Version: version}
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireToken(token, mcp.NewServer(info, mcptools.Instructions(), mcptools.New(p, assistant, hub, renderer)...)))
	if renderer != nil {
		mux.Handle("/mcp/render", mcp.RequireToken(token, mcp.NewServer(info, "", mcptools.Render(p, renderer)...)))
	}
	mux.Handle("/", server.New(p, assistant, hub, web, 500*time.Millisecond, renderer))

	fmt.Printf("hunsu: %s 를 %s 에서 엽니다 (LLM: %s)\n", p.Dir(), url, llmProvider.Name())
	fmt.Printf("MCP 서버: %smcp (헤더 Authorization: Bearer %s). 클라이언트별 등록 방법은 README 참고\n", url, token)
	if renderer == nil {
		fmt.Println("Chrome 을 찾지 못해 screenshot 도구를 끈다")
	}
	if openBrowser {
		exec.Command("open", url).Start()
	}
	return http.Serve(ln, mux)
}

// 브라우저 채팅을 처리할 LLM 을 고르는 유일한 곳이다. 다른 LLM 은 llm.Provider 를 구현한 뒤 여기에 더한다.
var providers = []struct {
	name string
	make func(path string, tools *llm.MCPServer) llm.Provider
}{
	{"claude", func(path string, tools *llm.MCPServer) llm.Provider { return claude.CLI{Path: path, Tools: tools} }},
	{"codex", func(path string, tools *llm.MCPServer) llm.Provider { return codex.CLI{Path: path, Tools: tools} }},
}

// name 이 비면 설치된 CLI 를 순서대로 찾는다. 아무것도 없으면 첫 provider 로 켜고, 요청할 때 실행 실패를 알린다.
func provider(name string, tools *llm.MCPServer) (llm.Provider, error) {
	for _, p := range providers {
		if name != "" && name != p.name {
			continue
		}
		path, err := exec.LookPath(p.name)
		if err == nil {
			return p.make(path, tools), nil
		}
		if name != "" {
			return nil, fmt.Errorf("%s 를 찾지 못했다. 설치하고 로그인했는지 확인한다", name)
		}
	}
	if name != "" {
		return nil, fmt.Errorf("모르는 provider: %s (claude, codex 중 하나)", name)
	}
	fmt.Println("claude 나 codex 를 찾지 못했다. 둘 중 하나를 설치하고 로그인해야 브라우저 채팅을 쓸 수 있다")
	return providers[0].make(providers[0].name, tools), nil
}

// 버전 썸네일은 첫 화면만 찍는다. Chrome 이 없으면 썸네일 없이 버전을 남긴다.
func thumbnailer(renderer mcptools.Renderer) history.Thumbnailer {
	if renderer == nil {
		return nil
	}
	return func(page string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return renderer.Screenshot(ctx, page, 1280, 800)
	}
}

// Chrome 이 없으면 nil 이다. 렌더 결과 확인 기능만 꺼지고 나머지는 그대로 동작한다.
func findRenderer(base string) mcptools.Renderer {
	path := render.Find()
	if path == "" {
		return nil
	}
	return render.Chrome{Path: path, Base: base}
}

func mcpToken() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return mcp.LoadToken(filepath.Join(configDir, "hunsu", "mcp-token"))
}
