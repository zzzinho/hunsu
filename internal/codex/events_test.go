package codex_test

import (
	"reflect"
	"testing"

	"hunsu/internal/codex"
	"hunsu/internal/llm"
)

func TestDecoder_Decode(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []llm.Event
	}{
		{"세션 시작을 세션 ID 와 함께 알린다",
			[]string{`{"type":"thread.started","thread_id":"t1"}`},
			[]llm.Event{{Kind: llm.Started, SessionID: "t1"}}},
		{"턴을 시작하거나 추론을 시작하면 생각하는 중으로 알린다",
			[]string{`{"type":"turn.started"}`, `{"type":"item.started","item":{"type":"reasoning"}}`},
			[]llm.Event{{Kind: llm.Thinking}, {Kind: llm.Thinking}}},
		{"파일 수정을 시작하면 파일 이름과 함께 쓰기로 알린다",
			[]string{`{"type":"item.started","item":{"type":"file_change","changes":[{"path":"/proj/a.html","kind":"update"}]}}`},
			[]llm.Event{{Kind: llm.Writing, File: "a.html"}}},
		{"명령 실행은 파일을 살펴보는 것으로 알린다",
			[]string{`{"type":"item.started","item":{"type":"command_execution","command":"cat a.html"}}`},
			[]llm.Event{{Kind: llm.Reading}}},
		{"MCP 도구를 쓰기 시작하면 도구 이름을 알린다",
			[]string{`{"type":"item.started","item":{"type":"mcp_tool_call","server":"hunsu","tool":"screenshot"}}`},
			[]llm.Event{{Kind: llm.UsingTool, Text: "screenshot"}}},
		{"명령이나 도구가 실패하면 재시도를 알린다",
			[]string{`{"type":"item.completed","item":{"type":"command_execution","exit_code":1,"status":"failed"}}`,
				`{"type":"item.completed","item":{"type":"mcp_tool_call","status":"failed"}}`},
			[]llm.Event{{Kind: llm.Retrying}, {Kind: llm.Retrying}}},
		{"성공한 명령은 따로 알리지 않는다",
			[]string{`{"type":"item.completed","item":{"type":"command_execution","exit_code":0,"status":"completed"}}`},
			nil},
		{"메시지는 답변 쓰는 중으로 알리고, 턴이 끝나면 마지막 메시지를 답변으로 전달한다",
			[]string{`{"type":"item.completed","item":{"type":"agent_message","text":"확인하겠습니다"}}`,
				`{"type":"item.completed","item":{"type":"agent_message","text":"고쳤습니다"}}`,
				`{"type":"turn.completed","usage":{}}`},
			[]llm.Event{{Kind: llm.Answering}, {Kind: llm.Answering}, {Kind: llm.Replied, Text: "고쳤습니다"}}},
		{"턴이 실패하면 이유와 함께 실패로 알린다",
			[]string{`{"type":"turn.failed","error":{"message":"모델을 쓸 수 없다"}}`},
			[]llm.Event{{Kind: llm.Failed, Text: "모델을 쓸 수 없다"}}},
		{"경고 항목과 JSON 이 아닌 줄은 무시한다",
			[]string{`{"type":"item.completed","item":{"type":"error","message":"metadata not found"}}`, "warning: something"},
			nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d codex.Decoder
			var got []llm.Event
			for _, line := range tt.lines {
				got = append(got, d.Decode([]byte(line))...)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decode() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
