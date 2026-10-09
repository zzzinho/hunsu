package mcptools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"hunsu/internal/chat"
	"hunsu/internal/claude"
	"hunsu/internal/mcp"
	"hunsu/internal/mcptools"
	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
	"hunsu/internal/ui"
)

type idleRunner struct{}

func (idleRunner) Run(context.Context, claude.Request, func(claude.Event)) error { return nil }

type fixture struct {
	t       *testing.T
	dir     string
	project *project.Project
	hub     *ui.Hub
	tools   []mcp.Tool
}

func newFixture(t *testing.T) *fixture {
	p, dir := projecttest.New(t, map[string]string{"a.html": "a", "design.css": ":root{}"})
	svc, err := chat.New(p, idleRunner{})
	if err != nil {
		t.Fatal(err)
	}
	hub := ui.NewHub()
	return &fixture{t: t, dir: dir, project: p, hub: hub, tools: mcptools.New(p, svc, hub)}
}

func (f *fixture) call(name, args string) string {
	for _, tool := range f.tools {
		if tool.Name == name {
			out, err := tool.Call(context.Background(), json.RawMessage(args))
			if err != nil {
				return "error: " + err.Error()
			}
			return out
		}
	}
	f.t.Fatalf("%s 도구가 없다", name)
	return ""
}

func (f *fixture) must(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}

func TestTools(t *testing.T) {
	tests := []struct {
		name string
		run  func(f *fixture) string
		want string
	}{
		{"get_context 는 디자인 폴더, 페이지, 디자인 시스템, 보고 있는 페이지, 선택한 요소를 알려준다",
			func(f *fixture) string {
				f.hub.Update(ui.State{Page: "a.html", Selection: &ui.Selection{Selector: "h1", HTML: "<h1>안녕</h1>"}})
				return strings.ReplaceAll(f.call("get_context", `{}`), f.dir, "<dir>")
			},
			`{"folder":"<dir>","pages":["a.html"],"designSystem":{"design.css":":root{}"},"currentPage":"a.html","selection":{"selector":"h1","html":"<h1>안녕</h1>"}}`},
		{"아무것도 선택하지 않았으면 selection 은 null 이다",
			func(f *fixture) string {
				return fmt.Sprint(strings.Contains(f.call("get_context", `{}`), `"selection":null`))
			},
			"true"},
		{"checkpoint 뒤에 undo 하면 그 시점으로 돌아간다",
			func(f *fixture) string {
				f.call("checkpoint", `{}`)
				f.must(f.project.Write("a.html", []byte("Claude Code 가 고침")))
				f.call("undo", `{}`)
				data, err := f.project.Read("a.html")
				f.must(err)
				return string(data)
			},
			"a"},
		{"되돌릴 시점이 없으면 undo 는 실패를 알린다",
			func(f *fixture) string { return f.call("undo", `{}`) },
			"error: 되돌릴 것이 없다"},
		{"open_page 는 연결된 브라우저에 페이지를 열라고 보낸다",
			func(f *fixture) string {
				inbox, cancel := f.hub.Subscribe()
				defer cancel()
				out := f.call("open_page", `{"page":"a.html"}`)
				command := <-inbox
				return out + " / " + command.Kind + ":" + command.Page
			},
			"브라우저 1곳에서 a.html 을 열었다 / open:a.html"},
		{"브라우저가 연결되어 있지 않으면 open_page 는 실패를 알린다",
			func(f *fixture) string { return f.call("open_page", `{"page":"a.html"}`) },
			"error: 연결된 브라우저가 없다. 사용자에게 hunsu 를 브라우저로 열어 달라고 한다"},
		{"없는 페이지는 열지 않는다",
			func(f *fixture) string { return f.call("open_page", `{"page":"none.html"}`) },
			"error: none.html 은 페이지가 아니다. get_context 의 pages 중에서 고른다"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.run(newFixture(t)); got != tt.want {
				t.Errorf("got %q\nwant %q", got, tt.want)
			}
		})
	}
}
