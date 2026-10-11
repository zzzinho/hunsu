package llm

import "cmp"

const (
	Started   = "start"
	Thinking  = "think"
	Reading   = "read"
	Writing   = "write"
	Answering = "answer"
	// UsingTool 은 hunsu 가 건넨 MCP 도구를 쓰는 중이다. Text 는 도구 이름이다.
	UsingTool = "tool"
	Retrying  = "retry"
	Replied   = "reply"
	Failed    = "fail"
)

type Event struct {
	Kind      string `json:"kind"`
	File      string `json:"file,omitempty"`
	Size      int    `json:"size,omitempty"`
	Text      string `json:"text,omitempty"`
	SessionID string `json:"-"`
}

func (e Event) String() string {
	if detail := cmp.Or(e.File, e.Text); detail != "" {
		return e.Kind + ":" + detail
	}
	return e.Kind
}
