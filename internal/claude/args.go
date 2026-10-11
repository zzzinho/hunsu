package claude

import (
	"encoding/json"

	"hunsu/internal/llm"
)

func Args(r llm.Request) []string {
	// 격리하지 않으면 사용자 설정·hook·MCP 를 불러와 2배 느리고 10배 비싸다 (PLAN.md 11).
	// --safe-mode 는 --mcp-config 로 넘긴 서버까지 끈다. 도구를 붙일 때는 --restricted 만으로 격리한다.
	args := []string{"-p"}
	if r.Tools == nil {
		args = append(args, "--safe-mode", "--strict-mcp-config")
	} else {
		args = append(args, "--strict-mcp-config", "--mcp-config", mcpConfig(r.Tools), "--allowedTools", "mcp__"+r.Tools.Name)
	}
	args = append(args,
		"--restricted", "--tools", "Read,Edit,Write", "--permission-mode", "acceptEdits",
		"--append-system-prompt", r.SystemPrompt,
		"--output-format", "stream-json", "--verbose",
		// 큰 파일은 다 쓰는 데 1분 넘게 걸린다. 그동안의 진행을 보여주려면 조각 단위 이벤트가 필요하다 (PLAN.md 11).
		"--include-partial-messages",
	)
	if r.SessionID != "" {
		args = append(args, "--resume", r.SessionID)
	}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	return args
}

func mcpConfig(s *llm.MCPServer) string {
	server := map[string]any{"type": "http", "url": s.URL, "headers": map[string]string{"Authorization": "Bearer " + s.Token}}
	config, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{s.Name: server}})
	return string(config)
}
