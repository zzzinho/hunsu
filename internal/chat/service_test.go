package chat_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hunsu/internal/chat"
	"hunsu/internal/history"
	"hunsu/internal/llm"
	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
	"hunsu/internal/ui"
)

type fakeProvider struct {
	requests []llm.Request
	writes   map[string]string
	err      error
	started  chan struct{}
	release  chan struct{}
}

func (f *fakeProvider) Models() []llm.Model {
	return []llm.Model{{ID: "fast", Label: "빠름"}, {ID: "slow", Label: "느림"}}
}

func (f *fakeProvider) Run(_ context.Context, req llm.Request, emit func(llm.Event)) error {
	f.requests = append(f.requests, req)
	emit(llm.Event{Kind: llm.Started, SessionID: "s1"})
	if f.release != nil {
		close(f.started)
		<-f.release
	}
	for name, body := range f.writes {
		if err := os.WriteFile(filepath.Join(req.Dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return f.err
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
	svc, err := chat.New(f.project, f.runner, nil)
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

func (f *fixture) lastRequest() llm.Request { return f.runner.requests[len(f.runner.requests)-1] }

func (f *fixture) versions() string {
	versions, err := f.svc.Versions()
	f.must(err)
	var out []string
	for _, v := range versions {
		out = append(out, fmt.Sprintf("%d:%s:%s", v.ID, v.Label, v.Page))
	}
	return strings.Join(out, " ")
}

func (f *fixture) read(name string) string {
	data, err := f.project.Read(name)
	f.must(err)
	return string(data)
}

func (f *fixture) warmUp() { f.send(chat.Turn{Prompt: "준비"}) }

const designBlock = "[디자인 시스템]\n<design.css>\n:root { --color-primary: red; }\n</design.css>\n"

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
			"[현재 페이지] a.html\n[요청] 더 크게"},
		{"열린 페이지가 없으면 요청만 보낸다",
			func(f *fixture) string {
				f.warmUp()
				f.send(chat.Turn{Prompt: "랜딩 페이지 만들어줘"})
				return f.lastRequest().Prompt
			},
			"[요청] 랜딩 페이지 만들어줘"},
		{"선택한 요소의 위치와 HTML 을 함께 보낸다",
			func(f *fixture) string {
				f.warmUp()
				f.send(chat.Turn{Page: "a.html", Prompt: "더 크게",
					Selection: &ui.Selection{Selector: "main > button.cta", HTML: `<button class="cta">시작</button>`}})
				return f.lastRequest().Prompt
			},
			"[현재 페이지] a.html\n[선택한 요소] main > button.cta\n<button class=\"cta\">시작</button>\n[요청] 더 크게"},
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
			"[알림] 마지막 응답 이후 사용자가 직접 고친 파일: a.html. 고치기 전에 다시 읽어라.\n[요청] 2"},
		{"대화의 첫 요청에는 디자인 시스템 전체를 요청 끝에 붙인다",
			func(f *fixture) string {
				f.send(chat.Turn{Page: "a.html", Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[현재 페이지] a.html\n" + designBlock + "[요청] 고쳐"},
		{"디자인 시스템이 없으면 첫 요청에 없다고 알린다",
			func(f *fixture) string {
				f.must(f.project.Remove("design.css"))
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[디자인 시스템] 아직 없다\n[요청] 고쳐"},
		{"바뀌지 않은 디자인 시스템은 다시 보내지 않는다",
			func(f *fixture) string {
				f.warmUp()
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[요청] 고쳐"},
		{"사용자가 고친 디자인 시스템 파일은 바뀐 내용을 붙인다",
			func(f *fixture) string {
				f.warmUp()
				f.must(f.project.Write("design.css", []byte(":root { --color-primary: blue; }")))
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[알림] 마지막 응답 이후 사용자가 직접 고친 파일: design.css. 고치기 전에 다시 읽어라.\n[디자인 시스템]\n<design.css>\n:root { --color-primary: blue; }\n</design.css>\n[요청] 고쳐"},
		{"LLM 이 직접 고친 디자인 시스템은 다시 보내지 않는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"design.css": ":root { --color-primary: green; }"}
				f.warmUp()
				f.runner.writes = nil
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			"[요청] 고쳐"},
		{"새 대화를 시작하면 첫 요청에 디자인 시스템 전체를 다시 붙인다",
			func(f *fixture) string {
				f.warmUp()
				f.must(f.svc.Reset())
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			designBlock + "[요청] 고쳐"},
		{"서버를 다시 켜면 첫 요청에 디자인 시스템 전체를 다시 붙인다",
			func(f *fixture) string {
				f.warmUp()
				f.restart()
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.lastRequest().Prompt
			},
			designBlock + "[요청] 고쳐"},
		{"사용자가 올린 에셋 목록을 요청마다 알린다",
			func(f *fixture) string {
				f.warmUp()
				f.must(f.project.AddAsset("logo.png", strings.NewReader("png")))
				f.send(chat.Turn{Prompt: "로고 넣어줘"})
				return f.lastRequest().Prompt
			},
			"[에셋] assets/logo.png\n[요청] 로고 넣어줘"},
		{"system prompt 는 디자인 시스템이 바뀌어도 그대로라 캐시를 깨지 않는다",
			func(f *fixture) string {
				f.warmUp()
				f.must(f.project.Write("design.css", []byte(":root { --color-primary: blue; }")))
				f.send(chat.Turn{Prompt: "고쳐"})
				return fmt.Sprint(f.runner.requests[0].SystemPrompt == f.lastRequest().SystemPrompt)
			},
			"true"},
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
			"2:버튼 둥글게:a.html 1:요청 전:a.html"},
		{"요청 전에 사용자가 고친 내용은 '요청 전' 버전으로 따로 남는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Page: "a.html", Prompt: "1"})
				f.must(f.project.Write("a.html", []byte("직접 고침")))
				f.send(chat.Turn{Page: "a.html", Prompt: "2"})
				return f.versions()
			},
			"4:2:a.html 3:요청 전:a.html 2:1:a.html 1:요청 전:a.html"},
		{"실패한 요청은 버전을 남기지 않는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "반쯤 고침"}
				f.runner.err = errors.New("boom")
				f.send(chat.Turn{Page: "a.html", Prompt: "고쳐"})
				return f.versions()
			},
			"1:요청 전:a.html"},
		{"새로 만든 페이지가 있으면 그 페이지를 버전의 대표 페이지로 쓴다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"b.html": "b"}
				f.send(chat.Turn{Page: "a.html", Prompt: "b 만들어"})
				return f.versions()
			},
			"2:b 만들어:b.html 1:요청 전:a.html"},
		{"보던 페이지가 없으면 첫 페이지를 대표 페이지로 쓴다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Prompt: "고쳐"})
				return f.versions()
			},
			"2:고쳐:a.html 1:요청 전:a.html"},
		{"버전으로 복원하면 그 파일로 돌아가고, 복원 전 상태와 복원 결과도 버전으로 남는다",
			func(f *fixture) string {
				f.runner.writes = map[string]string{"a.html": "LLM 이 고침"}
				f.send(chat.Turn{Page: "a.html", Prompt: "고쳐"})
				f.must(f.project.Write("a.html", []byte("직접 고침")))
				f.must(f.svc.Restore(1, "a.html"))
				return f.read("a.html") + " / " + f.versions()
			},
			"a / 4:v1 복원:a.html 3:복원 전:a.html 2:고쳐:a.html 1:요청 전:a.html"},
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
			"true false 1:직접 저장:a.html"},
		{"요청을 처리하는 동안 다른 요청, 되돌리기, 체크포인트, 버전 저장, 복원은 받지 않는다",
			func(f *fixture) string {
				f.runner.started, f.runner.release = make(chan struct{}), make(chan struct{})
				done := make(chan string)
				go func() { done <- f.send(chat.Turn{Prompt: "1"}) }()
				<-f.runner.started
				_, saveErr := f.svc.SaveVersion("a.html")
				got := fmt.Sprint(f.svc.Send(context.Background(), chat.Turn{Prompt: "2"}, func(llm.Event) {}), " / ", f.svc.Undo(), " / ", f.svc.Checkpoint(),
					" / ", saveErr, " / ", f.svc.Restore(1, "a.html"))
				close(f.runner.release)
				<-done
				return got
			},
			strings.Repeat(chat.ErrBusy.Error()+" / ", 4) + chat.ErrBusy.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.run(newFixture(t)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
