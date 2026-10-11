package codex

import (
	"encoding/json"
	"path/filepath"

	"hunsu/internal/llm"
)

type streamLine struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Error    struct {
		Message string `json:"message"`
	} `json:"error"`
	Item struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Tool     string `json:"tool"`
		Status   string `json:"status"`
		ExitCode *int   `json:"exit_code"`
		Changes  []struct {
			Path string `json:"path"`
		} `json:"changes"`
	} `json:"item"`
}

// codex 는 중간 메시지도 agent_message 로 보낸다. 턴이 끝날 때 마지막 메시지를 답변으로 쓴다. 요청 하나에 Decoder 하나를 쓴다.
type Decoder struct {
	lastMessage string
}

func (d *Decoder) Decode(raw []byte) []llm.Event {
	var line streamLine
	if json.Unmarshal(raw, &line) != nil {
		return nil
	}
	switch line.Type {
	case "thread.started":
		return []llm.Event{{Kind: llm.Started, SessionID: line.ThreadID}}
	case "turn.started":
		return []llm.Event{{Kind: llm.Thinking}}
	case "item.started":
		return started(line)
	case "item.completed":
		return d.completed(line)
	case "turn.completed":
		return []llm.Event{{Kind: llm.Replied, Text: d.lastMessage}}
	case "turn.failed":
		return []llm.Event{{Kind: llm.Failed, Text: line.Error.Message}}
	}
	return nil
}

func started(line streamLine) []llm.Event {
	item := line.Item
	switch item.Type {
	case "reasoning":
		return []llm.Event{{Kind: llm.Thinking}}
	case "command_execution":
		return []llm.Event{{Kind: llm.Reading}}
	case "mcp_tool_call":
		return []llm.Event{{Kind: llm.UsingTool, Text: item.Tool}}
	case "file_change":
		if len(item.Changes) > 0 {
			return []llm.Event{{Kind: llm.Writing, File: filepath.Base(item.Changes[0].Path)}}
		}
		return []llm.Event{{Kind: llm.Writing}}
	}
	return nil
}

func (d *Decoder) completed(line streamLine) []llm.Event {
	item := line.Item
	switch {
	case item.Type == "agent_message":
		d.lastMessage = item.Text
		return []llm.Event{{Kind: llm.Answering}}
	case item.Status == "failed":
		return []llm.Event{{Kind: llm.Retrying}}
	}
	return nil
}
