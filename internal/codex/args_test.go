package codex_test

import (
	"strings"
	"testing"

	"hunsu/internal/codex"
	"hunsu/internal/llm"
)

var hunsuTools = &llm.MCPServer{Name: "hunsu", URL: "http://127.0.0.1:7777/mcp/render", Token: "t"}

func TestArgs(t *testing.T) {
	tests := []struct {
		name    string
		req     llm.Request
		want    string
		notWant string
	}{
		{"새 대화는 exec 로 시작하고 진행을 JSONL 로 받는다", llm.Request{}, "exec --json", "resume"},
		{"이어가는 대화는 exec resume 으로 세션을 잇는다", llm.Request{SessionID: "t1"}, "exec resume --json", ""},
		{"git 저장소가 아닌 디자인 폴더에서도 실행한다", llm.Request{}, "--skip-git-repo-check", ""},
		{"사용자 설정, 실행 규칙, 프로젝트 AGENTS.md 를 섞지 않는다", llm.Request{}, "--ignore-user-config --ignore-rules -c project_doc_max_bytes=0", ""},
		{"명령은 디자인 폴더에만 쓸 수 있는 샌드박스에서 실행한다", llm.Request{}, `-c sandbox_mode="workspace-write"`, ""},
		{"지시문은 developer_instructions 로 넘긴다", llm.Request{SystemPrompt: "규칙\n\"따옴표\""}, `-c developer_instructions="규칙\n\"따옴표\""`, ""},
		{"hunsu 가 건넨 MCP 서버를 붙이고 그 도구를 자동으로 승인한다", llm.Request{Tools: hunsuTools},
			`-c mcp_servers.hunsu.url="http://127.0.0.1:7777/mcp/render" -c mcp_servers.hunsu.http_headers={Authorization="Bearer t"} -c mcp_servers.hunsu.default_tools_approval_mode="approve"`, ""},
		{"MCP 서버를 받지 않으면 붙이지 않는다", llm.Request{}, "", "mcp_servers"},
		{"모델을 고르면 그 모델을 쓴다", llm.Request{Model: "gpt-5.6-luna"}, "-m gpt-5.6-luna", ""},
		{"세션을 이으면 세션 ID 다음에 표준 입력을 프롬프트로 받는다", llm.Request{SessionID: "t1", Prompt: "비밀 요청"}, "t1 -", "비밀 요청"},
		{"프롬프트는 인자가 아니라 표준 입력으로 받는다", llm.Request{Prompt: "비밀 요청"}, "", "비밀 요청"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(codex.Args(tt.req), " ")
			if !strings.Contains(got, tt.want) || (tt.notWant != "" && strings.Contains(got, tt.notWant)) || !strings.HasSuffix(got, " -") {
				t.Errorf("Args() = %q; want %q 포함, %q 미포함, 표준 입력(-)으로 끝남", got, tt.want, tt.notWant)
			}
		})
	}
}
