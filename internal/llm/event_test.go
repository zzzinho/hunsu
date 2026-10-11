package llm_test

import (
	"testing"

	"hunsu/internal/llm"
)

func TestEvent_String(t *testing.T) {
	tests := []struct {
		name  string
		event llm.Event
		want  string
	}{
		{"종류만 있으면 종류만 보인다", llm.Event{Kind: llm.Started, SessionID: "s1"}, "start"},
		{"파일이 있으면 파일을 함께 보인다", llm.Event{Kind: llm.Writing, File: "a.html"}, "write:a.html"},
		{"글이 있으면 글을 함께 보인다", llm.Event{Kind: llm.Replied, Text: "고쳤습니다"}, "reply:고쳤습니다"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
