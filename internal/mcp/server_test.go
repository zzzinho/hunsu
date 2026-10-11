package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hunsu/internal/mcp"
)

var echo = mcp.Tool{
	Name:        "echo",
	Description: "글을 그대로 돌려준다",
	InputSchema: map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}},
	Call: func(_ context.Context, args json.RawMessage) (mcp.Result, error) {
		var in struct {
			Text string `json:"text"`
		}
		err := json.Unmarshal(args, &in)
		return mcp.Result{Text: in.Text}, err
	},
}

var picture = mcp.Tool{
	Name:        "picture",
	Description: "그림과 설명을 돌려준다",
	InputSchema: map[string]any{"type": "object"},
	Call: func(context.Context, json.RawMessage) (mcp.Result, error) {
		return mcp.Result{Image: []byte("png"), Text: "설명"}, nil
	},
}

var broken = mcp.Tool{
	Name:        "broken",
	Description: "항상 실패한다",
	InputSchema: map[string]any{"type": "object"},
	Call:        func(context.Context, json.RawMessage) (mcp.Result, error) { return mcp.Result{}, errors.New("고장") },
}

func rpc(method, params string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":` + params + `}`
}

func TestServer(t *testing.T) {
	h := mcp.NewServer(mcp.Info{Name: "hunsu", Version: "test"}, "안내문", echo, picture, broken)
	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
		wantBody   string
	}{
		{"initialize 에 요청한 프로토콜 버전, 도구 기능, 서버 정보, 안내문을 알린다", "POST",
			rpc("initialize", `{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"c","version":"1"}}`), 200,
			`"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"hunsu","version":"test"},"instructions":"안내문"}`},
		{"아는 이전 프로토콜 버전을 요청하면 그 버전으로 답한다", "POST",
			rpc("initialize", `{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"c","version":"1"}}`), 200,
			`"protocolVersion":"2024-11-05"`},
		{"모르는 프로토콜 버전을 요청하면 아는 최신 버전으로 답한다", "POST",
			rpc("initialize", `{"protocolVersion":"2099-01-01","capabilities":{},"clientInfo":{"name":"c","version":"1"}}`), 200,
			`"protocolVersion":"2025-06-18"`},
		{"ping 에 같은 id 와 빈 결과로 답한다", "POST", rpc("ping", `{}`), 200, `{"jsonrpc":"2.0","id":1,"result":{}}`},
		{"여러 요청을 배열로 한 번에 보내면 응답도 배열로 돌려준다", "POST",
			"[" + rpc("ping", `{}`) + "," + strings.Replace(rpc("ping", `{}`), `"id":1`, `"id":2`, 1) + "]", 200,
			`[{"jsonrpc":"2.0","id":1,"result":{}},{"jsonrpc":"2.0","id":2,"result":{}}]`},
		{"알림만 담은 배열에는 본문 없이 202 로 답한다", "POST", `[{"jsonrpc":"2.0","method":"notifications/initialized"}]`, 202, ""},
		{"알림에는 본문 없이 202 로 답한다", "POST", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, 202, ""},
		{"도구 목록을 이름, 설명, 입력 형식과 함께 알린다", "POST", rpc("tools/list", `{}`), 200,
			`{"name":"echo","description":"글을 그대로 돌려준다","inputSchema":{"properties":{"text":{"type":"string"}},"type":"object"}}`},
		{"도구를 호출해 결과를 텍스트로 돌려준다", "POST", rpc("tools/call", `{"name":"echo","arguments":{"text":"안녕"}}`), 200,
			`"result":{"content":[{"type":"text","text":"안녕"}]}`},
		{"도구가 돌려준 PNG 는 텍스트 앞에 이미지로 담는다", "POST", rpc("tools/call", `{"name":"picture","arguments":{}}`), 200,
			`"result":{"content":[{"type":"image","data":"cG5n","mimeType":"image/png"},{"type":"text","text":"설명"}]}`},
		{"도구가 실패하면 결과에 에러로 표시한다", "POST", rpc("tools/call", `{"name":"broken","arguments":{}}`), 200,
			`"result":{"content":[{"type":"text","text":"고장"}],"isError":true}`},
		{"없는 도구를 부르면 잘못된 인자 에러로 답한다", "POST", rpc("tools/call", `{"name":"none","arguments":{}}`), 200, `"error":{"code":-32602`},
		{"모르는 메서드는 메서드 없음 에러로 답한다", "POST", rpc("resources/list", `{}`), 200, `"error":{"code":-32601`},
		{"JSON 이 아니면 파싱 에러로 답한다", "POST", "{", 200, `"id":null,"error":{"code":-32700`},
		{"POST 가 아니면 받지 않는다", "GET", "", 405, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, "/mcp", strings.NewReader(tt.body)))
			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("%s = %d %s; want %d, %s 포함", tt.method, rec.Code, rec.Body, tt.wantStatus, tt.wantBody)
			}
		})
	}
}

func TestRequireToken(t *testing.T) {
	h := mcp.RequireToken("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tests := []struct {
		name          string
		authorization string
		want          int
	}{
		{"토큰이 맞으면 통과시킨다", "Bearer secret", 200},
		{"토큰이 틀리면 거부한다", "Bearer wrong", 401},
		{"토큰이 없으면 거부한다", "", 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/mcp", nil)
			req.Header.Set("Authorization", tt.authorization)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("상태 = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
