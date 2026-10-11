package mcptools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"hunsu/internal/chat"
	"hunsu/internal/llm"
	"hunsu/internal/mcp"
	"hunsu/internal/mcptools"
	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
	"hunsu/internal/ui"
)

type idleProvider struct{}

func (idleProvider) Models() []llm.Model { return nil }

func (idleProvider) Run(context.Context, llm.Request, func(llm.Event)) error { return nil }

type fakeRenderer struct{}

func (fakeRenderer) Screenshot(_ context.Context, page string, width, _ int) ([]byte, error) {
	return fmt.Appendf(nil, "%s@%d", page, width), nil
}

type fixture struct {
	t       *testing.T
	dir     string
	project *project.Project
	hub     *ui.Hub
	tools   []mcp.Tool
}

func newFixture(t *testing.T) *fixture {
	p, dir := projecttest.New(t, map[string]string{"a.html": "a", "design.css": ":root{}"})
	svc, err := chat.New(p, idleProvider{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	hub := ui.NewHub()
	return &fixture{t: t, dir: dir, project: p, hub: hub, tools: mcptools.New(p, svc, hub, fakeRenderer{})}
}

func (f *fixture) call(name, args string) string {
	for _, tool := range f.tools {
		if tool.Name == name {
			out, err := tool.Call(context.Background(), json.RawMessage(args))
			if err != nil {
				return "error: " + err.Error()
			}
			if out.Image != nil {
				return "image:" + string(out.Image) + " / " + out.Text
			}
			return out.Text
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
		{"get_context 는 디자인 폴더, 페이지, 에셋, 디자인 시스템, 보고 있는 페이지, 선택한 요소를 알려준다",
			func(f *fixture) string {
				f.must(f.project.AddAsset("logo.png", strings.NewReader("png")))
				f.hub.Update(ui.State{Page: "a.html", Selection: &ui.Selection{Selector: "h1", HTML: "<h1>안녕</h1>"}})
				return strings.ReplaceAll(f.call("get_context", `{}`), f.dir, "<dir>")
			},
			`{"folder":"<dir>","pages":["a.html"],"assets":["assets/logo.png"],"designSystem":{"design.css":":root{}"},"currentPage":"a.html","selection":{"selector":"h1","html":"<h1>안녕</h1>"}}`},
		{"아무것도 선택하지 않았으면 selection 은 null 이다",
			func(f *fixture) string {
				return fmt.Sprint(strings.Contains(f.call("get_context", `{}`), `"selection":null`))
			},
			"true"},
		{"checkpoint 뒤에 undo 하면 그 시점으로 돌아간다",
			func(f *fixture) string {
				f.call("checkpoint", `{}`)
				f.must(f.project.Write("a.html", []byte("에이전트가 고침")))
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
		{"screenshot 은 페이지를 요청한 너비로 렌더한 이미지를 돌려준다",
			func(f *fixture) string { return f.call("screenshot", `{"page":"a.html","width":390}`) },
			"image:a.html@390 / a.html, 너비 390px"},
		{"screenshot 은 없는 페이지를 찍지 않는다",
			func(f *fixture) string { return f.call("screenshot", `{"page":"none.html"}`) },
			"error: none.html 은 페이지가 아니다. get_context 의 pages 중에서 고른다"},
		{"Chrome 이 없으면 screenshot 도구를 내놓지 않는다",
			func(f *fixture) string {
				svc, err := chat.New(f.project, idleProvider{}, nil)
				f.must(err)
				var names []string
				for _, tool := range mcptools.New(f.project, svc, f.hub, nil) {
					names = append(names, tool.Name)
				}
				return strings.Join(names, ",")
			},
			"get_context,checkpoint,undo,open_page"},
		{"브라우저 채팅의 provider 에는 화면 확인 도구만 준다",
			func(f *fixture) string {
				var names []string
				for _, tool := range mcptools.Render(f.project, fakeRenderer{}) {
					names = append(names, tool.Name)
				}
				return strings.Join(names, ",")
			},
			"screenshot"},
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
