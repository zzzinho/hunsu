package claude_test

import (
	"strings"
	"testing"

	"hunsu/internal/claude"
	"hunsu/internal/llm"
)

func TestArgs(t *testing.T) {
	tests := []struct {
		name    string
		req     llm.Request
		want    string
		notWant string
	}{
		{"사용자 설정과 MCP 서버를 섞지 않는다", llm.Request{}, "--safe-mode --strict-mcp-config", ""},
		{"명령 실행 도구 없이 프로젝트 폴더의 파일만 읽고 쓴다", llm.Request{}, "--restricted --tools Read,Edit,Write --permission-mode acceptEdits", ""},
		{"지시문은 system prompt 뒤에 붙이고, 캐시가 깨지지 않게 대화 내내 그대로 쓴다", llm.Request{SystemPrompt: "규칙"}, "--append-system-prompt 규칙", "--system-prompt-snapshot"},
		{"진행 상황을 stream-json 으로 받는다", llm.Request{}, "--output-format stream-json --verbose", ""},
		{"파일을 다 쓰기 전에도 쓰는 중인 조각을 받는다", llm.Request{}, "--include-partial-messages", ""},
		{"세션이 있으면 이어간다", llm.Request{SessionID: "s1"}, "--resume s1", ""},
		{"세션이 없으면 새 대화로 시작한다", llm.Request{}, "", "--resume"},
		{"모델을 고르면 그 모델을 쓴다", llm.Request{Model: "haiku"}, "--model haiku", ""},
		{"모델을 고르지 않으면 claude 기본 모델을 쓴다", llm.Request{}, "", "--model"},
		{"프롬프트는 인자로 넘기지 않는다", llm.Request{Prompt: "비밀 요청"}, "", "비밀 요청"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(claude.Args(tt.req), " ")
			if !strings.Contains(got, tt.want) || (tt.notWant != "" && strings.Contains(got, tt.notWant)) {
				t.Errorf("Args() = %q; want %q 포함, %q 미포함", got, tt.want, tt.notWant)
			}
		})
	}
}
