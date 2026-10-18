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
	"properties": map[string]any{"page": map[string]any{"type": "string", "description": "File name of the page to open (e.g. index.html)"}},
	"required":   []string{"page"},
}

var errNoBrowser = errors.New("no browser is connected. Ask the user to open hunsu in a browser")

func Instructions() string { return guide + project.Rules }

func New(p *project.Project, assistant *chat.Service, hub *ui.Hub, renderer Renderer) []mcp.Tool {
	tools := []mcp.Tool{
		{
			Name:        "get_context",
			Description: "Tells you what the user sees in hunsu: the design folder's absolute path, pages, uploaded asset (image and font) paths, design system contents (DESIGN.md, design.css), the page visible in the browser, and the element selected in the browser (CSS selector and HTML). Call it when you start design work and when the user says \"this\" or \"the selected one\".",
			InputSchema: noArgs,
			Call:        func(context.Context, json.RawMessage) (mcp.Result, error) { return text(workspaceOf(p, hub)) },
		},
		{
			Name:        "checkpoint",
			Description: "Saves the current state before you edit design files. The user can return to it with Undo in the browser or the undo tool. Call it once per request, before you start editing files.",
			InputSchema: noArgs,
			Call: func(context.Context, json.RawMessage) (mcp.Result, error) {
				return mcp.Result{Text: "saved a point to undo to"}, assistant.Checkpoint()
			},
		},
		{
			Name:        "undo",
			Description: "Restores design files to the last checkpoint. It shares history with Undo in the browser, so if a browser chat request or a version restore happened after the checkpoint, it goes back to just before that first. Pages created since are removed.",
			InputSchema: noArgs,
			Call: func(context.Context, json.RawMessage) (mcp.Result, error) {
				return mcp.Result{Text: "restored the last checkpoint"}, assistant.Undo(hub.Current().Page)
			},
		},
		{
			Name:        "open_page",
			Description: "Opens a page in hunsu in the browser. Call it after creating a page or editing one that is not visible.",
			InputSchema: pageArg,
			Call:        func(_ context.Context, args json.RawMessage) (mcp.Result, error) { return text(openPage(p, hub, args)) },
		},
	}
	return append(tools, Render(p, renderer)...)
}

func text(s string, err error) (mcp.Result, error) { return mcp.Result{Text: s}, err }

type workspace struct {
	Folder       string            `json:"folder"`
	Pages        []string          `json:"pages"`
	Assets       []string          `json:"assets"`
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
	// 선택한 요소의 HTML 을 에이전트가 \u003c 같은 이스케이프 없이 그대로 읽게 한다.
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(workspace{Folder: p.Dir(), Pages: listing.Pages, Assets: listing.Assets, DesignSystem: design, CurrentPage: state.Page, Selection: state.Selection})
	return string(bytes.TrimSpace(out.Bytes())), err
}

func openPage(p *project.Project, hub *ui.Hub, args json.RawMessage) (string, error) {
	var in struct {
		Page string `json:"page"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	if err := requirePage(p, in.Page); err != nil {
		return "", err
	}
	sent := hub.Send(ui.Command{Kind: ui.Open, Page: in.Page})
	if sent == 0 {
		return "", errNoBrowser
	}
	return fmt.Sprintf("opened %[2]s in %[1]d browser(s)", sent, in.Page), nil
}

func requirePage(p *project.Project, page string) error {
	listing, err := p.List()
	if err != nil {
		return err
	}
	if !slices.Contains(listing.Pages, page) {
		return fmt.Errorf("%s is not a page. Pick one of get_context's pages", page)
	}
	return nil
}
