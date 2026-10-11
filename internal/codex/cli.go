package codex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"hunsu/internal/llm"
)

type CLI struct {
	Path string
}

// 첫 번째가 기본이다. 2026-10 에 ChatGPT 로그인으로 쓸 수 있음을 확인한 모델이다 (PLAN.md 11).
var models = []llm.Model{{ID: "gpt-5.6-sol", Label: "gpt-5.6-sol"}, {ID: "gpt-5.6-terra", Label: "gpt-5.6-terra"}, {ID: "gpt-5.6-luna", Label: "gpt-5.6-luna", Fast: true}}

func (CLI) Name() string { return "codex" }

func (CLI) Models() []llm.Model { return slices.Clone(models) }

func (c CLI) Status(ctx context.Context) llm.Status {
	if _, err := exec.LookPath(c.Path); err != nil {
		return llm.Status{Problem: llm.NotInstalled, Fix: "brew install --cask codex"}
	}
	if exec.CommandContext(ctx, c.Path, "login", "status").Run() != nil {
		return llm.Status{Problem: llm.NotLoggedIn, Fix: "codex login"}
	}
	return llm.Status{Ready: true}
}

func (c CLI) Run(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.Path, Args(req)...)
	cmd.Dir = req.Dir
	cmd.Stdin = strings.NewReader(req.Prompt)
	var decoder Decoder
	var failure string
	cmd.Stdout = llm.Lines(func(line []byte) {
		for _, e := range decoder.Decode(line) {
			if e.Kind == llm.Failed {
				failure = e.Text
			}
			emit(e)
		}
	})
	cmd.Stderr = &stderr
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codex failed: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	// 턴이 실패해도 종료 코드는 0 이다.
	if failure != "" {
		return errors.New(failure)
	}
	return nil
}
