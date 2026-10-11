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
	svc, err := chat.New(f.project, f.runner)
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
				f.send(chat.Turn{Page: "a.html", Prompt: "더 크게"})
				return f.lastRequest().Prompt
			},
			"[현재 페이지] a.html\n[요청] 더 크게"},
		{"열린 페이지가 없으면 요청만 보낸다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "랜딩 페이지 만들어줘"})
				return f.lastRequest().Prompt
			},
			"[요청] 랜딩 페이지 만들어줘"},
		{"선택한 요소의 위치와 HTML 을 함께 보낸다",
			func(f *fixture) string {
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
		{"디자인 시스템 내용을 system prompt 로 보낸다",
			func(f *fixture) string {
				f.send(chat.Turn{Prompt: "고쳐"})
				return fmt.Sprint(strings.Contains(f.lastRequest().SystemPrompt, "--color-primary: red"))
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
		{"요청을 처리하는 동안 다른 요청, 되돌리기, 체크포인트는 받지 않는다",
			func(f *fixture) string {
				f.runner.started, f.runner.release = make(chan struct{}), make(chan struct{})
				done := make(chan string)
				go func() { done <- f.send(chat.Turn{Prompt: "1"}) }()
				<-f.runner.started
				got := fmt.Sprint(f.svc.Send(context.Background(), chat.Turn{Prompt: "2"}, func(llm.Event) {}), " / ", f.svc.Undo(), " / ", f.svc.Checkpoint())
				close(f.runner.release)
				<-done
				return got
			},
			chat.ErrBusy.Error() + " / " + chat.ErrBusy.Error() + " / " + chat.ErrBusy.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.run(newFixture(t)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
