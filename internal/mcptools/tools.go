package mcptools

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"hunsu/internal/chat"
	"hunsu/internal/mcp"
	"hunsu/internal/project"
	"hunsu/internal/ui"
)

//go:embed instructions.md
var guide string

var noArgs = map[string]any{"type": "object", "properties": map[string]any{}}

var pageArg = map[string]any{
	"type":       "object",
	"properties": map[string]any{"page": map[string]any{"type": "string", "description": "열 페이지의 파일 이름 (예: index.html)"}},
	"required":   []string{"page"},
}

var errNoBrowser = errors.New("연결된 브라우저가 없다. 사용자에게 hunsu 를 브라우저로 열어 달라고 한다")

func Instructions() string { return guide + project.Rules }

func New(p *project.Project, assistant *chat.Service, hub *ui.Hub) []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "get_context",
			Description: "hunsu 에서 사용자가 보고 있는 것을 알려준다: 디자인 폴더의 절대 경로, 페이지 목록, 디자인 시스템(DESIGN.md, design.css) 내용, 브라우저에 보이는 페이지, 브라우저에서 선택한 요소(CSS 선택자와 HTML). 디자인 작업을 시작할 때와 사용자가 '이거', '선택한 것' 이라고 할 때 호출한다.",
			InputSchema: noArgs,
			Call:        func(context.Context, json.RawMessage) (string, error) { return workspaceOf(p, hub) },
		},
		{
			Name:        "checkpoint",
			Description: "디자인 파일을 고치기 전에 지금 상태를 저장한다. 사용자는 브라우저의 [되돌리기] 나 undo 도구로 이 시점에 돌아갈 수 있다. 요청 하나에서 파일을 고치기 시작하기 전에 한 번 호출한다.",
			InputSchema: noArgs,
			Call: func(context.Context, json.RawMessage) (string, error) {
				return "되돌릴 시점을 저장했다", assistant.Checkpoint()
			},
		},
		{
			Name:        "undo",
			Description: "마지막 checkpoint 시점으로 디자인 파일을 되돌린다. 그 뒤에 생긴 페이지는 지운다.",
			InputSchema: noArgs,
			Call: func(context.Context, json.RawMessage) (string, error) {
				return "마지막 checkpoint 시점으로 되돌렸다", assistant.Undo()
			},
		},
		{
			Name:        "open_page",
			Description: "브라우저의 hunsu 화면에 페이지를 연다. 새 페이지를 만들었거나 지금 보이지 않는 페이지를 고쳤을 때 호출한다.",
			InputSchema: pageArg,
			Call:        func(_ context.Context, args json.RawMessage) (string, error) { return openPage(p, hub, args) },
		},
	}
}

type workspace struct {
	Folder       string            `json:"folder"`
	Pages        []string          `json:"pages"`
	DesignSystem map[string]string `json:"designSystem"`
	CurrentPage  string            `json:"currentPage"`
	Selection    *ui.Selection     `json:"selection"`
}

func workspaceOf(p *project.Project, hub *ui.Hub) (string, error) {
	listing, err := p.List()
	if err != nil {
		return "", err
	}
	design := map[string]string{}
	for _, entry := range listing.DesignSystem {
		if data, err := p.Read(entry.Path); err == nil {
			design[entry.Path] = string(data)
		}
	}
	state := hub.Current()
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	// 선택한 요소의 HTML 을 Claude 가 < 대신 그대로 읽을 수 있게 한다.
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(workspace{Folder: p.Dir(), Pages: listing.Pages, DesignSystem: design, CurrentPage: state.Page, Selection: state.Selection})
	return string(bytes.TrimSpace(out.Bytes())), err
}

func openPage(p *project.Project, hub *ui.Hub, args json.RawMessage) (string, error) {
	var in struct {
		Page string `json:"page"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	listing, err := p.List()
	if err != nil {
		return "", err
	}
	if !slices.Contains(listing.Pages, in.Page) {
		return "", fmt.Errorf("%s 은 페이지가 아니다. get_context 의 pages 중에서 고른다", in.Page)
	}
	sent := hub.Send(ui.Command{Kind: ui.Open, Page: in.Page})
	if sent == 0 {
		return "", errNoBrowser
	}
	return fmt.Sprintf("브라우저 %d곳에서 %s 을 열었다", sent, in.Page), nil
}
