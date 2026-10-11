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
		"page":  map[string]any{"type": "string", "description": "찍을 페이지의 파일 이름 (예: index.html)"},
		"width": map[string]any{"type": "integer", "description": "화면 너비 px. 기본 1280, 모바일은 390"},
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
		Description: "디자인 페이지를 브라우저로 렌더한 이미지(위에서부터 최대 4000px)를 돌려준다. 새 페이지를 만들었거나 레이아웃을 크게 바꾼 뒤 결과를 눈으로 확인할 때 쓴다. 작은 수정에는 쓰지 않는다.",
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
		err = fmt.Errorf("%s 이 %s 안에 다 그려지지 않았다", in.Page, shotTimeout)
	}
	return mcp.Result{Image: png, Text: fmt.Sprintf("%s, 너비 %dpx", in.Page, width)}, err
}
