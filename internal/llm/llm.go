package llm

import "context"

// Provider 는 브라우저 채팅의 요청을 처리하는 LLM 이다. PoC 는 Claude Code CLI(claude -p) 를 쓰고,
// Anthropic API·Codex 같은 다른 LLM 은 이 인터페이스를 구현해 main 에서 바꿔 끼운다.
type Provider interface {
	// Run 은 req.Dir 의 디자인 파일을 직접 고치고, 진행 상황을 emit 으로 알린다.
	Run(ctx context.Context, req Request, emit func(Event)) error
	// Models 는 사용자가 고를 수 있는 모델이다. 첫 번째가 기본이다.
	Models() []Model
}

type Request struct {
	Dir          string
	Prompt       string
	SystemPrompt string
	// SessionID 는 provider 가 Started 이벤트로 알려준 대화 ID 다. 비어 있으면 새 대화를 시작한다.
	SessionID string
	Model     string
}

type Model struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
