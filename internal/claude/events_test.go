package claude_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"hunsu/internal/claude"
	"hunsu/internal/llm"
)

func blockStart(block string) string {
	return `{"type":"stream_event","event":{"type":"content_block_start","content_block":` + block + `}}`
}

func toolStart(name string) string { return blockStart(`{"type":"tool_use","name":"` + name + `"}`) }

func inputDelta(partial string) string {
	quoted, _ := json.Marshal(partial)
	return `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":` + string(quoted) + `}}}`
}

const (
	blockStop  = `{"type":"stream_event","event":{"type":"content_block_stop"}}`
	pathPrefix = `{"file_path":"/proj/a.html","content":"`
)

func TestDecoder_Decode(t *testing.T) {
	big := strings.Repeat("x", 1100)
	tests := []struct {
		name  string
		lines []string
		want  []llm.Event
	}{
		{"세션 시작을 세션 ID 와 함께 알린다",
			[]string{`{"type":"system","subtype":"init","session_id":"s1"}`},
			[]llm.Event{{Kind: llm.Started, SessionID: "s1"}}},
		{"생각을 시작하면 알린다",
			[]string{blockStart(`{"type":"thinking"}`)},
			[]llm.Event{{Kind: llm.Thinking}}},
		{"파일 읽기를 시작하면 알리고, 조각난 경로가 완성되면 파일 이름을 알린다",
			[]string{toolStart("Read"), inputDelta(`{"file_path":"/proj/a`), inputDelta(`.html"}`)},
			[]llm.Event{{Kind: llm.Reading}, {Kind: llm.Reading, File: "a.html"}}},
		{"Edit 는 파일 쓰기로 알린다",
			[]string{toolStart("Edit"), inputDelta(pathPrefix)},
			[]llm.Event{{Kind: llm.Writing}, {Kind: llm.Writing, File: "a.html"}}},
		{"Write 는 파일 쓰기로 알린다",
			[]string{toolStart("Write"), inputDelta(pathPrefix)},
			[]llm.Event{{Kind: llm.Writing}, {Kind: llm.Writing, File: "a.html"}}},
		{"파일을 쓰는 동안 1KB 늘어날 때마다 쓴 양을 알린다",
			[]string{toolStart("Write"), inputDelta(pathPrefix), inputDelta("xx"), inputDelta(big)},
			[]llm.Event{{Kind: llm.Writing}, {Kind: llm.Writing, File: "a.html"},
				{Kind: llm.Writing, File: "a.html", Size: len(pathPrefix) + 2 + len(big)}}},
		{"도구 블록이 끝나면 다음 입력 조각을 이어 붙이지 않는다",
			[]string{toolStart("Write"), blockStop, inputDelta(pathPrefix)},
			[]llm.Event{{Kind: llm.Writing}}},
		{"답변을 쓰기 시작하면 알린다",
			[]string{blockStart(`{"type":"text"}`)},
			[]llm.Event{{Kind: llm.Answering}}},
		{"도구 실행이 끝나면 다음 단계를 생각하는 중으로 알린다",
			[]string{`{"type":"user","message":{"content":[{"type":"tool_result"}]}}`},
			[]llm.Event{{Kind: llm.Thinking}}},
		{"도구가 실패하면 재시도를 알린다",
			[]string{`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true}]}}`},
			[]llm.Event{{Kind: llm.Retrying}}},
		{"성공한 결과는 답변으로 전달한다",
			[]string{`{"type":"result","subtype":"success","is_error":false,"result":"고쳤습니다"}`},
			[]llm.Event{{Kind: llm.Replied, Text: "고쳤습니다"}}},
		{"실패한 결과는 실패로 알린다",
			[]string{`{"type":"result","subtype":"error_max_turns","is_error":true}`},
			[]llm.Event{{Kind: llm.Failed, Text: "error_max_turns"}}},
		{"완성된 assistant 메시지는 부분 이벤트와 겹치므로 무시한다",
			[]string{`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"/proj/a.html"}}]}}`},
			nil},
		{"JSON 이 아닌 줄은 무시한다",
			[]string{"warning: something"},
			nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d claude.Decoder
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
