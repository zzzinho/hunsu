package mcptools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"hunsu/internal/mcp"
	"hunsu/internal/project"
	"hunsu/internal/render"
)

const (
	shotTimeout = 30 * time.Second
	// 아주 긴 페이지는 LLM 에 보낼 때 어차피 줄어들어 알아볼 수 없다. 위에서부터 이만큼만 찍는다.
	shotHeight = 4000
)

// Renderer 는 디자인 페이지를 PNG 로 찍는다.
type Renderer interface {
	Screenshot(ctx context.Context, page string, width, maxHeight int) ([]byte, error)
}

var shotArgs = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"page":  map[string]any{"type": "string", "description": "File name of the page to capture (e.g. index.html)"},
		"width": map[string]any{"type": "integer", "description": "Viewport width in px. Default 1280; use 390 for mobile"},
	},
	"required": []string{"page"},
}

// Render 는 화면 확인 도구다. 외부 에이전트와 브라우저 채팅의 provider 가 함께 쓴다. renderer 가 nil(Chrome 없음)이면 비어 있다.
func Render(p *project.Project, renderer Renderer) []mcp.Tool {
	if renderer == nil {
		return nil
	}
	return []mcp.Tool{{
		Name:        "screenshot",
		Description: "Returns an image of the design page rendered in a browser (top 4000 px at most). Use it to check the result after creating a page or substantially changing a layout. Do not use it for small edits.",
		InputSchema: shotArgs,
		Call: func(ctx context.Context, args json.RawMessage) (mcp.Result, error) {
			return screenshot(ctx, p, renderer, args)
		},
	}}
}

func screenshot(ctx context.Context, p *project.Project, renderer Renderer, args json.RawMessage) (mcp.Result, error) {
	var in struct {
		Page  string `json:"page"`
		Width int    `json:"width"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return mcp.Result{}, err
	}
	width, err := render.Width(in.Width)
	if err != nil {
		return mcp.Result{}, err
	}
	if err := requirePage(p, in.Page); err != nil {
		return mcp.Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, shotTimeout)
	defer cancel()
	png, err := renderer.Screenshot(ctx, in.Page, width, shotHeight)
	if errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("%s did not finish rendering within %s", in.Page, shotTimeout)
	}
	return mcp.Result{Image: png, Text: fmt.Sprintf("%s, %dpx wide", in.Page, width)}, err
}
