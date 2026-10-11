package claude

import "hunsu/internal/llm"

func Args(r llm.Request) []string {
	args := []string{
		"-p",
		// 격리하지 않으면 사용자 설정·hook·MCP 를 불러와 2배 느리고 10배 비싸다 (PLAN.md 14).
		"--safe-mode", "--strict-mcp-config",
		"--restricted", "--tools", "Read,Edit,Write", "--permission-mode", "acceptEdits",
		"--append-system-prompt", r.SystemPrompt,
		// 기본값(on)이면 resume 할 때 처음 디자인 규칙이 계속 쓰인다.
		"--system-prompt-snapshot", "off",
		"--output-format", "stream-json", "--verbose",
		// 큰 파일은 다 쓰는 데 1분 넘게 걸린다. 그동안의 진행을 보여주려면 조각 단위 이벤트가 필요하다 (PLAN.md 16).
		"--include-partial-messages",
	}
	if r.SessionID != "" {
		args = append(args, "--resume", r.SessionID)
	}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	return args
}
