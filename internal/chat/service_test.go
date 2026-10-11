package chat_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"hunsu/internal/chat"
	"hunsu/internal/history"
	"hunsu/internal/llm"
	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
	"hunsu/internal/ui"
)

// writes·reply 의 {dir} 은 실행한 폴더 이름(비교안 번호)으로 바뀐다. failIn 폴더에서 실행하면 실패한다.
// noStart 면 시작 이벤트 없이 err 로 끝난다. release 가 있으면 첫 실행이 started 를 닫고 release 를 기다린다.
type fakeProvider struct {
	mu        sync.Mutex
	startOnce sync.Once
	noStart   bool
	name      string
	requests  []llm.Request
	writes    map[string]string
	reply     string
	err       error
	failIn    string
	started   chan struct{}
	release   chan struct{}
}

func (f *fakeProvider) Status(context.Context) llm.Status { return llm.Status{Ready: true} }

func (f *fakeProvider) Name() string { return cmp.Or(f.name, "fake") }

func (f *fakeProvider) Models() []llm.Model {
	return []llm.Model{{ID: "fast", Label: "빠름"}, {ID: "slow", Label: "느림"}}
}

func (f *fakeProvider) Run(_ context.Context, req llm.Request, emit func(llm.Event)) error {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	if f.noStart {
		return f.err
	}
	emit(llm.Event{Kind: llm.Started, SessionID: "s1"})
	if f.release != nil {
		f.startOnce.Do(func() { close(f.started) })
		<-f.release
	}
	dir := filepath.Base(req.Dir)
	if dir == f.failIn {
		return errors.New("안 " + dir + " 실패")
	}
	for name, body := range f.writes {
		if err := os.WriteFile(filepath.Join(req.Dir, name), []byte(strings.ReplaceAll(body, "{dir}", dir)), 0o644); err != nil {
			return err
		}
	}
	if f.reply != "" {
		emit(llm.Event{Kind: llm.Replied, Text: strings.ReplaceAll(f.reply, "{dir}", dir)})
	}
	return f.err
}

// 비교안 요청은 동시에 실행되어 순서가 정해지지 않으므로 실행한 폴더 이름순으로 돌려준다.
func (f *fixture) variantRequests(n int) []llm.Request {
	requests := slices.Clone(f.runner.requests[len(f.runner.requests)-n:])
	slices.SortFunc(requests, func(a, b llm.Request) int { return strings.Compare(a.Dir, b.Dir) })
	return requests
}

func (f *fixture) sendVariants(turn chat.Turn, count int) string {
	var mu sync.Mutex
	var events []string
	err := f.svc.SendVariants(context.Background(), turn, count, func(e llm.Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, fmt.Sprintf("%d/%s", e.Variant, e))
	})
	if err != nil {
		events = append(events, "error:"+err.Error())
	}
	slices.Sort(events)
	return strings.Join(events, " ")
}

func (f *fixture) variants() string {
	var out []string
	for _, v := range f.svc.Variants() {
		out = append(out, fmt.Sprintf("%d:%s:%s:%s", v.ID, v.Page, v.Reply, v.Error))
	}
	return strings.Join(out, " ")
}

type fixture struct {
	t       *testing.T
	svc     *chat.Service
	runner  *fakeProvider
	project *project.Project
}

func newFixture(t *testing.T) *fixture {
	p, _ := projecttest.New(t, map[string]string{"a.html": "a", "design.css": ":root { --color-primary: red; }"})
	f := &fixture{t: t, runner: &fakeProvider{}, project: p}
	f.restart()
	return f
}

func (f *fixture) restart() {
	svc, err := chat.New(f.project, f.runner, chat.Options{Tools: hunsuTools})
	f.must(err)
	f.svc = svc
}

func (f *fixture) send(turn chat.Turn) string {
	var events []string
	err := f.svc.Send(context.Background(), turn, func(e llm.Event) { events = append(events, e.String()) })
	if err != nil {
		events = append(events, "error:"+err.Error())
	}
	return strings.Join(events, " ")
}

var hunsuTools = &llm.MCPServer{Name: "hunsu", URL: "http://127.0.0.1:7777/mcp/render", Token: "t"}

func (f *fixture) lastRequest() llm.Request { return f.runner.requests[len(f.runner.requests)-1] }

func (f *fixture) versions() string {
	versions, err := f.svc.Versions()
	f.must(err)
	var out []string
	for _, v := range versions {
		label := v.Kind
		if v.Text != "" {
			label += " " + v.Text
		}
		if v.Ref != 0 {
			label += fmt.Sprintf("#%d", v.Ref)
		}
		out = append(out, fmt.Sprintf("%d:%s:%s", v.ID, label, v.Page))
	}
	return strings.Join(out, " ")
}

func (f *fixture) read(name string) string {
	data, err := f.project.Read(name)
	f.must(err)
	return string(data)
}

func (f *fixture) warmUp() { f.send(chat.Turn{Prompt: "준비"}) }

const designBlock = "[Design system]\n<design.css>\n:root { --color-primary: red; }\n</design.css>\n"

func (f *fixture) must(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}

func TestService(t *testing.T) {
	tests := []struct {
		name string
		run  func(f *fixture) string
		want string
	}{
		{"provider 의 진행 이벤트를 전달하고 마지막에 끝을 알린다",
			func(f *fixture) string { return f.send(chat.Turn{Prompt: "고쳐"}) },
			"start end"},
		{"LLM 이 새로 만든 페이지를 알린다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"b.html": "b"}
				return f.send(chat.Turn{Prompt: "b 페이지 만들어"})
			},
			"start created:b.html end"},
		{"provider 가 실패하면 에러를 돌려준다",
			func(f *fixture) string {
				f.runner.err = errors.New("boom")
				return f.send(chat.Turn{Prompt: "고쳐"})
			},
			"start error:boom"},
		{"빈 요청은 보내지 않는다",
			func(f *fixture) string { return f.send(chat.Turn{Prompt: "  "}) },
			"error:" + chat.ErrEmptyPrompt.Error()},
		{"현재 페이지와 요청을 프롬프트로 보낸다",
			func(f *fixture) string {
				f.warmUp()
				f.send(chat.Turn{Page: "a.html", Prompt: "더 크게"})
				return f.lastRequest().Prompt
			},
			"[Current page] a.html\n[Request] 더 크게"},
		{"열린 페이지가 없으면 요청만 보낸다",
			func(f *fixture) string {
				f.warmUp()
				f.send(chat.Turn{Prompt: "랜딩 페이지 만들어줘"})
				return f.lastRequest().Prompt
			},
			"[Request] 랜딩 페이지 만들어줘"},
		{"선택한 요소의 위치와 HTML 을 함께 보낸다",
			func(f *fixture) string {
				f.warmUp()
				f.send(chat.Turn{Page: "a.html", Prompt: "더 크게",
					Selection: &ui.Selection{Selector: "main > button.cta", HTML: `<button class="cta">시작</button>`}})
				return f.lastRequest().Prompt
			},
			"[Current page] a.html\n[Selected element] main > button.cta\n<button class=\"cta\">시작</button>\n[Request] 더 크게"},
		{"선택한 요소의 HTML 이 길면 앞의 2000자만 보낸다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "고쳐", Selection: &ui.Selection{Selector: "body", HTML: strings.Repeat("가", 2100)}})
				return fmt.Sprint(strings.Count(f.lastRequest().Prompt, "가"))
			},
			"2000"},
		{"마지막 응답 이후 사용자가 직접 고친 파일을 알린다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "1"})
				f.must(f.project.Write("a.html", []byte("직접 고침")))
				f.send(chat.Turn{Prompt: "2"})
				return f.lastRequest().Prompt
			},
			"[Notice] Files the user edited since your last reply: a.html. Re-read them before editing.\n[Request] 2"},
		{"대화의 첫 요청에는 디자인 시스템 전체를 요청 끝에 붙인다",
			func(f *fixture) string {
				f.send(chat.Turn{Page: "a.html", Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[Current page] a.html\n" + designBlock + "[Request] 고쳐"},
		{"디자인 시스템이 없으면 첫 요청에 없다고 알린다",
			func(f *fixture) string {
				f.must(f.project.Remove("design.css"))
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[Design system] none yet\n[Request] 고쳐"},
		{"바뀌지 않은 디자인 시스템은 다시 보내지 않는다",
			func(f *fixture) string {
				f.warmUp()
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[Request] 고쳐"},
		{"사용자가 지운 디자인 파일도 고친 파일로 알린다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "1"})
				f.must(f.project.Remove("design.css"))
				f.send(chat.Turn{Prompt: "2"})
				return f.lastRequest().Prompt
			},
			"[Notice] Files the user edited since your last reply: design.css. Re-read them before editing.\n[Request] 2"},
		{"provider 가 시작하지 못한 요청 뒤에는 새 대화이므로 디자인 시스템을 다시 붙인다",
			func(f *fixture) string {
				f.runner.noStart, f.runner.err = true, errors.New("not logged in")
				f.send(chat.Turn{Prompt: "1"})
				f.runner.noStart, f.runner.err = false, nil
				f.send(chat.Turn{Prompt: "2"})
				return f.lastRequest().Prompt
			},
			designBlock + "[Request] 2"},
		{"사용자가 고친 디자인 시스템 파일은 바뀐 내용을 붙인다",
			func(f *fixture) string {
				f.warmUp()
				f.must(f.project.Write("design.css", []byte(":root { --color-primary: blue; }")))
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[Notice] Files the user edited since your last reply: design.css. Re-read them before editing.\n[Design system]\n<design.css>\n:root { --color-primary: blue; }\n</design.css>\n[Request] 고쳐"},
		{"LLM 이 직접 고친 디자인 시스템은 다시 보내지 않는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"design.css": ":root { --color-primary: green; }"}
				f.warmUp()
				f.runner.writes = nil
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[Request] 고쳐"},
		{"새 대화를 시작하면 첫 요청에 디자인 시스템 전체를 다시 붙인다",
			func(f *fixture) string {
				f.warmUp()
				f.must(f.svc.Reset())
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			designBlock + "[Request] 고쳐"},
		{"서버를 다시 켜면 첫 요청에 디자인 시스템 전체를 다시 붙인다",
			func(f *fixture) string {
				f.warmUp()
				f.restart()
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			designBlock + "[Request] 고쳐"},
		{"사용자가 올린 에셋 목록을 요청마다 알린다",
			func(f *fixture) string {
				f.warmUp()
				f.must(f.project.AddAsset("logo.png", strings.NewReader("png")))
				f.send(chat.Turn{Prompt: "로고 넣어줘"})
				return f.lastRequest().Prompt
			},
			"[Assets] assets/logo.png\n[Request] 로고 넣어줘"},
		{"system prompt 는 디자인 시스템이 바뀌어도 그대로라 캐시를 깨지 않는다",
			func(f *fixture) string {
				f.warmUp()
				f.must(f.project.Write("design.css", []byte(":root { --color-primary: blue; }")))
				f.send(chat.Turn{Prompt: "고쳐"})
				return fmt.Sprint(f.runner.requests[0].SystemPrompt == f.lastRequest().SystemPrompt)
			},
			"true"},
		{"hunsu 가 준비한 도구를 요청에 건넨다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Tools.URL
			},
			hunsuTools.URL},
		{"provider 의 이름과 지금 쓸 수 있는지를 알려준다",
			func(f *fixture) string {
				status := f.svc.Status(context.Background())
				return fmt.Sprint(status.Provider, " ", status.Ready)
			},
			"fake true"},
		{"고른 모델로 보낸다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "고쳐", Model: "slow"})
				return f.lastRequest().Model
			},
			"slow"},
		{"모델을 고르지 않으면 provider 의 기본(첫 번째) 모델로 보낸다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Model
			},
			"fast"},
		{"첫 응답의 세션을 다음 요청에서 이어간다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "1"})
				f.send(chat.Turn{Prompt: "2"})
				return f.lastRequest().SessionID
			},
			"s1"},
		{"서버를 다시 켜도 세션을 이어간다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "1"})
				f.restart()
				f.send(chat.Turn{Prompt: "2"})
				return f.lastRequest().SessionID
			},
			"s1"},
		{"provider 가 바뀌면 다른 provider 의 대화를 잇지 않는다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "1"})
				f.runner.name = "other"
				f.restart()
				f.send(chat.Turn{Prompt: "2"})
				return f.lastRequest().SessionID
			},
			""},
		{"새 대화를 시작하면 세션을 잇지 않는다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "1"})
				f.must(f.svc.Reset())
				f.send(chat.Turn{Prompt: "2"})
				return f.lastRequest().SessionID
			},
			""},
		{"되돌리기는 마지막 요청 전 상태로 돌린다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Prompt: "고쳐"})
				f.must(f.svc.Undo())
				data, err := f.project.Read("a.html")
				f.must(err)
				return string(data)
			},
			"a"},
		{"되돌려도 사용자가 올린 에셋은 남긴다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "고쳐"})
				f.must(f.project.AddAsset("logo.png", strings.NewReader("png")))
				f.must(f.svc.Undo())
				listing, err := f.project.List()
				f.must(err)
				return strings.Join(listing.Assets, ",")
			},
			"assets/logo.png"},
		{"요청이 끝나면 요청 내용을 이름으로, 그 전 상태는 '요청 전' 으로 버전을 남긴다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Page: "a.html", Prompt: "버튼 둥글게"})
				return f.versions()
			},
			"2:request 버튼 둥글게:a.html 1:before-request:a.html"},
		{"요청 전에 사용자가 고친 내용은 '요청 전' 버전으로 따로 남는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Page: "a.html", Prompt: "1"})
				f.must(f.project.Write("a.html", []byte("직접 고침")))
				f.send(chat.Turn{Page: "a.html", Prompt: "2"})
				return f.versions()
			},
			"4:request 2:a.html 3:before-request:a.html 2:request 1:a.html 1:before-request:a.html"},
		{"실패한 요청은 버전을 남기지 않는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "반쯤 고침"}
				f.runner.err = errors.New("boom")
				f.send(chat.Turn{Page: "a.html", Prompt: "고쳐"})
				return f.versions()
			},
			"1:before-request:a.html"},
		{"새로 만든 페이지가 있으면 그 페이지를 버전의 대표 페이지로 쓴다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"b.html": "b"}
				f.send(chat.Turn{Page: "a.html", Prompt: "b 만들어"})
				return f.versions()
			},
			"2:request b 만들어:b.html 1:before-request:a.html"},
		{"보던 페이지가 없으면 첫 페이지를 대표 페이지로 쓴다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.versions()
			},
			"2:request 고쳐:a.html 1:before-request:a.html"},
		{"버전으로 복원하면 그 파일로 돌아가고, 복원 전 상태와 복원 결과도 버전으로 남는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Page: "a.html", Prompt: "고쳐"})
				f.must(f.project.Write("a.html", []byte("직접 고침")))
				f.must(f.svc.Restore(1, "a.html"))
				return f.read("a.html") + " / " + f.versions()
			},
			"a / 4:restore#1:a.html 3:before-restore:a.html 2:request 고쳐:a.html 1:before-request:a.html"},
		{"없는 버전으로는 복원하지 않고 아무것도 남기지 않는다",
			func(f *fixture) string {
				err := f.svc.Restore(9, "a.html")
				return fmt.Sprint(errors.Is(err, history.ErrNoVersion), " ", errors.Is(f.svc.Undo(), history.ErrNothingToUndo), " [", f.versions(), "]")
			},
			"true true []"},
		{"복원은 되돌리기로 취소할 수 있다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Page: "a.html", Prompt: "고쳐"})
				f.must(f.svc.Restore(1, "a.html"))
				f.must(f.svc.Undo())
				return f.read("a.html")
			},
			"LLM 이 고침"},
		{"버전 저장은 지금 상태를 남기고, 바뀐 것이 없으면 남기지 않는다",
			func(f *fixture) string {
				first, err := f.svc.SaveVersion("a.html")
				f.must(err)
				second, err := f.svc.SaveVersion("a.html")
				f.must(err)
				return fmt.Sprint(first, " ", second, " ", f.versions())
			},
			"true false 1:saved:a.html"},
		{"비교안은 안마다 디자인 파일 복사본에서 도구 없이 새 대화로 실행한다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "준비"})
				f.sendVariants(chat.Turn{Prompt: "둥글게"}, 2)
				var out []string
				for _, r := range f.variantRequests(2) {
					out = append(out, fmt.Sprint(r.Dir == f.project.StatePath("variants", filepath.Base(r.Dir)), r.Tools == nil, r.SessionID == ""))
				}
				return strings.Join(out, " ")
			},
			"true true true true true true"},
		{"안마다 다른 방향을 붙이고, 새 대화라 디자인 시스템 전체를 함께 보낸다",
			func(f *fixture) string {
				f.warmUp()
				f.sendVariants(chat.Turn{Prompt: "둥글게"}, 3)
				var out []string
				for _, r := range f.variantRequests(3) {
					line := r.Prompt[strings.Index(r.Prompt, "[Variant]"):strings.Index(r.Prompt, "[Request]")]
					out = append(out, fmt.Sprint(strings.Contains(r.Prompt, designBlock), " ", strings.TrimSpace(line)))
				}
				return strings.Join(out, "\n")
			},
			"true [Variant] Take 1 of 3. Follow the request as faithfully as possible. Make it clearly different from the other takes.\n" +
				"true [Variant] Take 2 of 3. Solve the same request with bolder color, type and layout. Make it clearly different from the other takes.\n" +
				"true [Variant] Take 3 of 3. Solve the same request in a simpler, more restrained way. Make it clearly different from the other takes."},
		{"비교안 진행 이벤트에 안 번호를 붙이고, 모두 끝나면 끝을 알린다",
			func(f *fixture) string { return f.sendVariants(chat.Turn{Prompt: "둥글게"}, 2) },
			"0/end 1/start 2/start"},
		{"비교안은 원본 디자인 파일을 바꾸지 않는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "안 {dir}"}
				f.sendVariants(chat.Turn{Prompt: "둥글게"}, 2)
				return f.read("a.html")
			},
			"a"},
		{"안마다 대표 페이지와 답변, 실패 이유를 알려준다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"b.html": "안 {dir}"}
				f.runner.reply = "안 {dir} 완성"
				f.runner.failIn = "2"
				f.sendVariants(chat.Turn{Page: "a.html", Prompt: "b 만들어"}, 2)
				return f.variants()
			},
			"1:b.html:안 1 완성: 2:a.html::안 2 실패"},
		{"안을 고르면 그 안이 지금 상태가 되고, 나머지 안도 버전에 남는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "안 {dir}"}
				f.sendVariants(chat.Turn{Page: "a.html", Prompt: "둥글게"}, 2)
				f.must(f.svc.ChooseVariant(2, "a.html"))
				return f.read("a.html") + " / " + f.versions() + " / [" + f.variants() + "]"
			},
			"안 2 / 3:chosen 둥글게#2:a.html 2:variant 둥글게#1:a.html 1:before-request:a.html / []"},
		{"안을 고른 뒤 되돌리면 고르기 전으로 돌아간다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "안 {dir}"}
				f.sendVariants(chat.Turn{Page: "a.html", Prompt: "둥글게"}, 2)
				f.must(f.svc.ChooseVariant(1, "a.html"))
				f.must(f.svc.Undo())
				return f.read("a.html")
			},
			"a"},
		{"실패한 안은 고를 수 없다",
			func(f *fixture) string {
				f.runner.failIn = "2"
				f.sendVariants(chat.Turn{Prompt: "둥글게"}, 2)
				return fmt.Sprint(errors.Is(f.svc.ChooseVariant(2, ""), history.ErrNoVariant))
			},
			"true"},
		{"새 비교안을 만들다 취소하면 이전 비교안도 고를 수 없다",
			func(f *fixture) string {
				f.sendVariants(chat.Turn{Prompt: "둥글게"}, 2)
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				err := f.svc.SendVariants(ctx, chat.Turn{Prompt: "다시"}, 2, func(llm.Event) {})
				return fmt.Sprint(errors.Is(err, context.Canceled), " [", f.variants(), "]")
			},
			"true []"},
		{"비교안을 만드는 동안에도 고를 안 목록을 막힘 없이 읽는다",
			func(f *fixture) string {
				f.sendVariants(chat.Turn{Prompt: "처음"}, 2)
				f.runner.started, f.runner.release = make(chan struct{}), make(chan struct{})
				done := make(chan string)
				go func() { done <- f.sendVariants(chat.Turn{Prompt: "둥글게"}, 2) }()
				<-f.runner.started
				during := f.variants()
				close(f.runner.release)
				<-done
				return "[" + during + "] " + fmt.Sprint(len(f.svc.Variants()))
			},
			"[] 2"},
		{"안을 고르다 실패하면 고르기 전으로 되돌리고, 다시 고를 수 있다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "안 {dir}"}
				f.sendVariants(chat.Turn{Page: "a.html", Prompt: "둥글게"}, 2)
				unreadable := f.project.StatePath("variants", "1", "design.css")
				f.must(os.Chmod(unreadable, 0))
				err := f.svc.ChooseVariant(2, "a.html")
				afterFailure := f.read("a.html")
				f.must(os.Chmod(unreadable, 0o644))
				f.must(f.svc.ChooseVariant(2, "a.html"))
				return fmt.Sprint(err != nil, " ", afterFailure, " / ", f.read("a.html"))
			},
			"true a / 안 2"},
		{"비교안을 버리면 고를 것이 없다",
			func(f *fixture) string {
				f.sendVariants(chat.Turn{Prompt: "둥글게"}, 2)
				f.must(f.svc.DiscardVariants())
				return fmt.Sprint(errors.Is(f.svc.ChooseVariant(1, ""), chat.ErrNoVariants), " [", f.variants(), "]")
			},
			"true []"},
		{"비교안은 2개나 3개만 만든다",
			func(f *fixture) string {
				return f.sendVariants(chat.Turn{Prompt: "둥글게"}, 1) + " / " + f.sendVariants(chat.Turn{Prompt: "둥글게"}, 4)
			},
			"error:" + chat.ErrVariantCount.Error() + " / error:" + chat.ErrVariantCount.Error()},
		{"요청을 처리하는 동안 다른 요청, 되돌리기, 체크포인트, 버전 저장, 복원, 비교안 만들기·고르기·버리기는 받지 않는다",
			func(f *fixture) string {
				f.runner.started, f.runner.release = make(chan struct{}), make(chan struct{})
				done := make(chan string)
				go func() { done <- f.send(chat.Turn{Prompt: "1"}) }()
				<-f.runner.started
				_, saveErr := f.svc.SaveVersion("a.html")
				got := fmt.Sprint(f.svc.Send(context.Background(), chat.Turn{Prompt: "2"}, func(llm.Event) {}), " / ", f.svc.Undo(), " / ", f.svc.Checkpoint(),
					" / ", saveErr, " / ", f.svc.Restore(1, "a.html"),
					" / ", f.svc.SendVariants(context.Background(), chat.Turn{Prompt: "3"}, 2, func(llm.Event) {}),
					" / ", f.svc.ChooseVariant(1, ""), " / ", f.svc.DiscardVariants())
				close(f.runner.release)
				<-done
				return got
			},
			strings.Repeat(chat.ErrBusy.Error()+" / ", 7) + chat.ErrBusy.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.run(newFixture(t)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
