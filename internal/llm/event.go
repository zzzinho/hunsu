package llm

import "cmp"

const (
	Started   = "start"
	Thinking  = "think"
	Reading   = "read"
	Writing   = "write"
	Answering = "answer"
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
