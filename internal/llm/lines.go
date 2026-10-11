package llm

import (
	"bytes"
	"io"
)

// Lines 는 받은 출력을 줄 단위로 onLine 에 넘긴다. JSONL 로 진행을 알리는 CLI provider 가 쓴다.
func Lines(onLine func([]byte)) io.Writer { return &lineWriter{onLine: onLine} }

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
