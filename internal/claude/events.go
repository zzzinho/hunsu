package claude

import (
	"cmp"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"hunsu/internal/llm"
)

// 파일을 쓰는 동안 이만큼 늘 때마다 쓴 양을 알린다.
const sizeStep = 1024

var (
	toolKinds       = map[string]string{"Read": llm.Reading, "Edit": llm.Writing, "Write": llm.Writing}
	blockKinds      = map[string]string{"thinking": llm.Thinking, "text": llm.Answering}
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

func (d *Decoder) Decode(raw []byte) []llm.Event {
	var line streamLine
	if json.Unmarshal(raw, &line) != nil {
		return nil
	}
	switch line.Type {
	case "system":
		if line.Subtype == "init" {
			return []llm.Event{{Kind: llm.Started, SessionID: line.SessionID}}
		}
	case "stream_event":
		return d.streamEvent(line)
	case "user":
		return toolResults(line)
	case "result":
		if line.IsError {
			return []llm.Event{{Kind: llm.Failed, Text: cmp.Or(line.Result, line.Subtype)}}
		}
		return []llm.Event{{Kind: llm.Replied, Text: line.Result}}
	}
	return nil
}

func (d *Decoder) streamEvent(line streamLine) []llm.Event {
	e := line.Event
	switch e.Type {
	case "content_block_start":
		if parts := strings.SplitN(e.ContentBlock.Name, "__", 3); len(parts) == 3 && parts[0] == "mcp" && e.ContentBlock.Type == "tool_use" {
			return []llm.Event{{Kind: llm.UsingTool, Text: parts[2]}}
		}
		if kind, ok := toolKinds[e.ContentBlock.Name]; ok && e.ContentBlock.Type == "tool_use" {
			d.tool = &toolInput{kind: kind}
			return []llm.Event{{Kind: kind}}
		}
		if kind, ok := blockKinds[e.ContentBlock.Type]; ok {
			return []llm.Event{{Kind: kind}}
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

func (t *toolInput) append(partial string) []llm.Event {
	t.json = append(t.json, partial...)
	if t.file == "" {
		match := filePathPattern.FindSubmatch(t.json)
		if match == nil {
			return nil
		}
		t.file = filepath.Base(string(match[1]))
		t.reported = len(t.json)
		return []llm.Event{{Kind: t.kind, File: t.file}}
	}
	if t.kind == llm.Writing && len(t.json)-t.reported >= sizeStep {
		t.reported = len(t.json)
		return []llm.Event{{Kind: t.kind, File: t.file, Size: len(t.json)}}
	}
	return nil
}

// 사용자 메시지의 content 는 블록 배열이 아니라 문자열일 수도 있다. 그 경우 블록이 없는 것으로 본다.
func toolResults(line streamLine) []llm.Event {
	var blocks []struct {
		Type    string `json:"type"`
		IsError bool   `json:"is_error"`
	}
	json.Unmarshal(line.Message.Content, &blocks)
	var events []llm.Event
	for _, b := range blocks {
		switch {
		case b.Type != "tool_result":
		case b.IsError:
			events = append(events, llm.Event{Kind: llm.Retrying})
		default:
			events = append(events, llm.Event{Kind: llm.Thinking})
		}
	}
	return events
}
