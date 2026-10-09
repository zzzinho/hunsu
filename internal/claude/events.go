package claude

import (
	"cmp"
	"encoding/json"
	"path/filepath"
	"regexp"
)

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

const sizeStep = 1024

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

var (
	toolKinds       = map[string]string{"Read": Reading, "Edit": Writing, "Write": Writing}
	blockKinds      = map[string]string{"thinking": Thinking, "text": Answering}
	filePathPattern = regexp.MustCompile(`"file_path"\s*:\s*"([^"]+)"`)
)

type streamLine struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	Event     struct {
		Type         string `json:"type"`
		ContentBlock struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content_block"`
		Delta struct {
			Type        string `json:"type"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
	} `json:"event"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// 도구 입력은 여러 조각으로 나뉘어 오므로 이어 붙여야 한다. 요청 하나에 Decoder 하나를 쓴다.
type Decoder struct {
	tool *toolInput
}

type toolInput struct {
	kind     string
	json     []byte
	file     string
	reported int
}

func (d *Decoder) Decode(raw []byte) []Event {
	var line streamLine
	if json.Unmarshal(raw, &line) != nil {
		return nil
	}
	switch line.Type {
	case "system":
		if line.Subtype == "init" {
			return []Event{{Kind: Started, SessionID: line.SessionID}}
		}
	case "stream_event":
		return d.streamEvent(line)
	case "user":
		return toolResults(line)
	case "result":
		if line.IsError {
			return []Event{{Kind: Failed, Text: cmp.Or(line.Result, line.Subtype)}}
		}
		return []Event{{Kind: Replied, Text: line.Result}}
	}
	return nil
}

func (d *Decoder) streamEvent(line streamLine) []Event {
	e := line.Event
	switch e.Type {
	case "content_block_start":
		if kind, ok := toolKinds[e.ContentBlock.Name]; ok && e.ContentBlock.Type == "tool_use" {
			d.tool = &toolInput{kind: kind}
			return []Event{{Kind: kind}}
		}
		if kind, ok := blockKinds[e.ContentBlock.Type]; ok {
			return []Event{{Kind: kind}}
		}
	case "content_block_delta":
		if d.tool != nil && e.Delta.Type == "input_json_delta" {
			return d.tool.append(e.Delta.PartialJSON)
		}
	case "content_block_stop":
		d.tool = nil
	}
	return nil
}

func (t *toolInput) append(partial string) []Event {
	t.json = append(t.json, partial...)
	if t.file == "" {
		match := filePathPattern.FindSubmatch(t.json)
		if match == nil {
			return nil
		}
		t.file = filepath.Base(string(match[1]))
		t.reported = len(t.json)
		return []Event{{Kind: t.kind, File: t.file}}
	}
	if t.kind == Writing && len(t.json)-t.reported >= sizeStep {
		t.reported = len(t.json)
		return []Event{{Kind: t.kind, File: t.file, Size: len(t.json)}}
	}
	return nil
}

// 사용자 메시지의 content 는 블록 배열이 아니라 문자열일 수도 있다. 그 경우 블록이 없는 것으로 본다.
func toolResults(line streamLine) []Event {
	var blocks []struct {
		Type    string `json:"type"`
		IsError bool   `json:"is_error"`
	}
	json.Unmarshal(line.Message.Content, &blocks)
	var events []Event
	for _, b := range blocks {
		switch {
		case b.Type != "tool_result":
		case b.IsError:
			events = append(events, Event{Kind: Retrying})
		default:
			events = append(events, Event{Kind: Thinking})
		}
	}
	return events
}
