package claude

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CLI struct {
	Path string
}

func (c CLI) Run(ctx context.Context, req Request, emit func(Event)) error {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.Path, req.Args()...)
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
