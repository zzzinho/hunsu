package claude

import (
	"bytes"
	"context"
	"encoding/json"
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

// 첫 번째가 기본이다. 사용자의 claude 기본 모델 설정과 관계없이 디자인 품질이 가장 좋은 opus 를 쓴다.
var models = []llm.Model{{ID: "opus", Label: "opus"}, {ID: "sonnet", Label: "sonnet"}, {ID: "haiku", Label: "haiku", Fast: true}}

func (CLI) Name() string { return "claude" }

func (CLI) Models() []llm.Model { return slices.Clone(models) }

func (c CLI) Status(ctx context.Context) llm.Status {
	if _, err := exec.LookPath(c.Path); err != nil {
		return llm.Status{Problem: llm.NotInstalled, Fix: "brew install --cask claude-code"}
	}
	out, _ := exec.CommandContext(ctx, c.Path, "auth", "status", "--json").Output()
	var auth struct {
		LoggedIn bool `json:"loggedIn"`
	}
	if json.Unmarshal(out, &auth) != nil || !auth.LoggedIn {
		return llm.Status{Problem: llm.NotLoggedIn, Fix: "claude auth login"}
	}
	return llm.Status{Ready: true}
}

func (c CLI) Run(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.Path, Args(req)...)
	cmd.Dir = req.Dir
	cmd.Stdin = strings.NewReader(req.Prompt)
	var decoder Decoder
	cmd.Stdout = llm.Lines(func(line []byte) {
		for _, e := range decoder.Decode(line) {
			emit(e)
		}
	})
	cmd.Stderr = &stderr
	// 취소 뒤에 claude 의 하위 프로세스가 출력 파이프를 붙잡고 있어도 기다리지 않는다.
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude failed: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return nil
}
