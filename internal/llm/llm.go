package llm

import "context"

// Provider 는 브라우저 채팅의 요청을 처리하는 LLM 이다. Claude Code CLI·Codex CLI 가 구현하고,
// Anthropic API 같은 다른 LLM 도 이 인터페이스를 구현해 main 에 더한다.
type Provider interface {
	// Name 은 provider 를 구별하는 이름이다. 대화 ID 를 provider 별로 저장하는 데 쓴다.
	Name() string
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

// MCPServer 는 hunsu 가 provider 에게 건네는 도구 서버(MCP Streamable HTTP, Bearer 토큰)다.
// provider 는 생성할 때 받아 LLM 이 그 도구를 쓸 수 있게 붙인다.
type MCPServer struct {
	Name  string
	URL   string
	Token string
}

type Model struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
