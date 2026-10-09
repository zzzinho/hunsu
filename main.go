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
	"time"

	"hunsu/internal/chat"
	"hunsu/internal/claude"
	"hunsu/internal/project"
	"hunsu/internal/server"
)

//go:embed web
var webFiles embed.FS

func main() {
	noOpen := flag.Bool("no-open", false, "브라우저를 자동으로 열지 않는다")
	flag.Parse()
	if err := run(cmp.Or(flag.Arg(0), "."), !*noOpen); err != nil {
		fmt.Fprintln(os.Stderr, "hunsu:", err)
		os.Exit(1)
	}
}

func run(dir string, openBrowser bool) error {
	p, err := project.Open(dir)
	if err != nil {
		return err
	}
	defer p.Close()
	web, err := fs.Sub(webFiles, "web")
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Printf("hunsu: %s 를 %s 에서 엽니다\n", dir, url)
	if openBrowser {
		exec.Command("open", url).Start()
	}
	assistant, err := chat.New(p, claude.CLI{Path: "claude"})
	if err != nil {
		return err
	}
	return http.Serve(ln, server.New(p, assistant, web, 500*time.Millisecond))
}
