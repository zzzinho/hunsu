package main

import (
	"cmp"
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"hunsu/internal/chat"
	"hunsu/internal/claude"
	"hunsu/internal/codex"
	"hunsu/internal/history"
	"hunsu/internal/llm"
	"hunsu/internal/mcp"
	"hunsu/internal/mcptools"
	"hunsu/internal/ports"
	"hunsu/internal/project"
	"hunsu/internal/render"
	"hunsu/internal/server"
	"hunsu/internal/ui"
)

// 클론한 폴더에서 go build 하면 Go 가 git 커밋으로 만든 버전(고친 채면 +dirty)을 넣는다.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "dev"
}

//go:embed web
var webFiles embed.FS

func main() {
	noOpen := flag.Bool("no-open", false, "do not open the browser")
	port := flag.Int("port", 0, "server port for this run (default: the folder's own port, starting at 7777)")
	providerName := flag.String("provider", "", "LLM for the browser chat: claude or codex. Defaults to the first installed of claude, codex")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("hunsu", version())
		return
	}
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
	claim, running, err := p.Claim()
	if errors.Is(err, project.ErrRunning) {
		fmt.Printf("hunsu: already running for %s at %s\n", p.Dir(), running)
		if openBrowser && running != "" {
			exec.Command("open", running).Start()
		}
		return nil
	}
	if err != nil {
		return err
	}
	defer claim.Close()
	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		return err
	}
	token, err := mcpToken()
	if err != nil {
		return err
	}
	ln, err := listen(p.Dir(), port)
	if err != nil {
		return fmt.Errorf("%w (choose another port with -port)", err)
	}
	url := "http://" + ln.Addr().String() + "/"
	if err := claim.Announce(url); err != nil {
		return err
	}
	renderer := findRenderer(url + "p/")
	var tools *llm.MCPServer
	if renderer != nil {
		tools = &llm.MCPServer{Name: "hunsu", URL: url + "mcp/render", Token: token}
	}
	llmProvider, err := provider(providerName)
	if err != nil {
		return err
	}
	assistant, err := chat.New(p, llmProvider, chat.Options{Thumbnail: thumbnailer(renderer), Tools: tools})
	if err != nil {
		return err
	}
	hub := ui.NewHub()

	info := mcp.Info{Name: "hunsu", Version: version()}
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireToken(token, mcp.NewServer(info, mcptools.Instructions(), mcptools.New(p, assistant, hub, renderer)...)))
	if renderer != nil {
		mux.Handle("/mcp/render", mcp.RequireToken(token, mcp.NewServer(info, "", mcptools.Render(p, renderer)...)))
	}
	mux.Handle("/", server.New(p, assistant, hub, web, 500*time.Millisecond, renderer))

	fmt.Printf("hunsu: serving %s at %s (LLM: %s)\n", p.Dir(), url, llmProvider.Name())
	fmt.Printf("MCP server: %smcp (header Authorization: Bearer %s). See the README to register it with your client\n", url, token)
	if renderer == nil {
		fmt.Println("Chrome not found: screenshots, PNG export and version thumbnails are off")
	}
	go func() {
		if status := assistant.Status(context.Background()); !status.Ready {
			fmt.Printf("%s is %s. Run: %s\n", status.Provider, strings.ReplaceAll(status.Problem, "-", " "), status.Fix)
		}
	}()
	if openBrowser {
		exec.Command("open", url).Start()
	}
	return http.Serve(ln, server.Guard(mux))
}

// 브라우저 채팅을 처리할 LLM 을 고르는 유일한 곳이다. 다른 LLM 은 llm.Provider 를 구현한 뒤 여기에 더한다.
var providers = []struct {
	name string
	make func(path string) llm.Provider
}{
	{"claude", func(path string) llm.Provider { return claude.CLI{Path: path} }},
	{"codex", func(path string) llm.Provider { return codex.CLI{Path: path} }},
}

// name 이 비면 설치된 CLI 를 순서대로 찾는다. 아무것도 없으면 첫 provider 로 켜고, 상태 확인이 설치 방법을 알린다.
func provider(name string) (llm.Provider, error) {
	for _, p := range providers {
		if name != "" && name != p.name {
			continue
		}
		path, err := exec.LookPath(p.name)
		if err == nil {
			return p.make(path), nil
		}
		if name != "" {
			return nil, fmt.Errorf("%s not found. Install it and log in", name)
		}
	}
	if name != "" {
		return nil, fmt.Errorf("unknown provider %s (use claude or codex)", name)
	}
	return providers[0].make(providers[0].name), nil
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

// port 가 0 이면 폴더에 정해 둔 포트를 쓴다. -port 로 준 포트는 그 실행에만 쓰고 기억하지 않는다.
func listen(dir string, port int) (net.Listener, error) {
	if port != 0 {
		return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	}
	registry, err := configPath("ports.json")
	if err != nil {
		return nil, err
	}
	return ports.Listen(registry, dir, ports.First)
}

func mcpToken() (string, error) {
	path, err := configPath("mcp-token")
	if err != nil {
		return "", err
	}
	return mcp.LoadToken(path)
}

func configPath(name string) (string, error) {
	configDir, err := os.UserConfigDir()
	return filepath.Join(configDir, "hunsu", name), err
}
