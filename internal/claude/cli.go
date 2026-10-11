package claude

import (
	"bytes"
	"context"
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
var models = []llm.Model{{ID: "opus", Label: "opus"}, {ID: "sonnet", Label: "sonnet"}, {ID: "haiku", Label: "haiku (빠름)"}}

func (CLI) Models() []llm.Model { return slices.Clone(models) }

func (c CLI) Run(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.Path, Args(req)...)
	cmd.Dir = req.Dir
	cmd.Stdin = strings.NewReader(req.Prompt)
	var decoder Decoder
	cmd.Stdout = &lineWriter{onLine: func(line []byte) {
		for _, e := range decoder.Decode(line) {
			emit(e)
		}
	}}
	cmd.Stderr = &stderr
	// 취소 뒤에 claude 의 하위 프로세스가 출력 파이프를 붙잡고 있어도 기다리지 않는다.
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude 실행 실패: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return nil
}

type lineWriter struct {
	buf    []byte
	onLine func([]byte)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		w.onLine(w.buf[:i])
		w.buf = w.buf[i+1:]
	}
}
