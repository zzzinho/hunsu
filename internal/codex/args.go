package codex

import (
	"encoding/json"

	"hunsu/internal/llm"
)

func Args(r llm.Request) []string {
	args := []string{"exec"}
	if r.SessionID != "" {
		args = append(args, "resume")
	}
	args = append(args, "--json", "--skip-git-repo-check",
		// 사용자 설정·규칙·AGENTS.md 를 섞지 않는다. 전역 ~/.codex/AGENTS.md 는 끌 방법이 없다 (PLAN.md 11).
		"--ignore-user-config", "--ignore-rules", "-c", "project_doc_max_bytes=0",
		// exec resume 에는 --sandbox 가 없어서 설정으로 넘긴다.
		"-c", `sandbox_mode="workspace-write"`,
		"-c", "developer_instructions="+tomlString(r.SystemPrompt),
	)
	if r.Tools != nil {
		server := "mcp_servers." + r.Tools.Name
		args = append(args,
			"-c", server+".url="+tomlString(r.Tools.URL),
			"-c", server+".http_headers={Authorization="+tomlString("Bearer "+r.Tools.Token)+"}",
			// exec 는 승인을 물을 수 없어서, 승인하지 않은 MCP 도구 호출은 거부된다.
			"-c", server+`.default_tools_approval_mode="approve"`,
		)
	}
	if r.Model != "" {
		args = append(args, "-m", r.Model)
	}
	if r.SessionID != "" {
		args = append(args, r.SessionID)
	}
	return append(args, "-")
}

// JSON 문자열은 TOML 기본 문자열로도 읽힌다. -c 의 값은 TOML 로 해석된다.
func tomlString(s string) string {
	quoted, _ := json.Marshal(s)
	return string(quoted)
}
