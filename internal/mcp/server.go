package mcp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"slices"
)

const (
	codeParse          = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// 도구 호출만 쓰므로 이 버전들은 주고받는 형식이 같다. 첫 번째가 최신이다.
var versions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Call        func(ctx context.Context, args json.RawMessage) (Result, error)
}

// Result 는 도구 호출 결과다. Image 는 PNG 다.
type Result struct {
	Text  string
	Image []byte
}

type Server struct {
	info         Info
	instructions string
	tools        []Tool
}

func NewServer(info Info, instructions string, tools ...Tool) *Server {
	return &Server{info: info, instructions: instructions, tools: tools}
}

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Tools struct{} `json:"tools"`
	} `json:"capabilities"`
	ServerInfo   Info   `json:"serverInfo"`
	Instructions string `json:"instructions"`
}

type toolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

func (r Result) content() []content {
	var out []content
	if r.Image != nil {
		out = append(out, content{Type: "image", Data: r.Image, MimeType: "image/png"})
	}
	if r.Text != "" || r.Image == nil {
		out = append(out, content{Type: "text", Text: r.Text})
	}
	return out
}

type callResult struct {
	Content []content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Streamable HTTP 의 응답 하나짜리 형태만 쓴다. 서버가 먼저 보낼 메시지가 없어서 SSE 스트림은 열지 않는다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "only POST is accepted", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: &rpcError{codeParse, err.Error()}})
		return
	}
	if out := s.reply(r.Context(), body); out != nil {
		writeJSON(w, out)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// ServeStdio 는 표준 입출력으로 MCP 를 주고받는다. 한 줄에 메시지 하나다. r 이 끝나면 nil 을 돌려준다.
func (s *Server) ServeStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, maxMessage)
	encoder := json.NewEncoder(w)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if out := s.reply(ctx, line); out != nil {
			if err := encoder.Encode(out); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

const maxMessage = 16 << 20

// reply 는 메시지 하나에 대한 답이다. 알림에는 답하지 않으므로 답할 것이 없으면 nil 이다.
// 2025-03-26 은 여러 요청을 배열로 묶어 보낼 수 있다(batch). 그때는 답도 배열이다.
func (s *Server) reply(ctx context.Context, body []byte) any {
	var requests []request
	var err error
	batch := len(bytes.TrimSpace(body)) > 0 && bytes.TrimSpace(body)[0] == '['
	if batch {
		err = json.Unmarshal(body, &requests)
	} else {
		requests = make([]request, 1)
		err = json.Unmarshal(body, &requests[0])
	}
	if err != nil {
		return response{JSONRPC: "2.0", Error: &rpcError{codeParse, err.Error()}}
	}
	var responses []response
	for _, req := range requests {
		if req.ID == nil {
			continue
		}
		result, rpcErr := s.handle(ctx, req)
		responses = append(responses, response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr})
	}
	switch {
	case len(responses) == 0:
		return nil
	case batch:
		return responses
	}
	return responses[0]
}

func (s *Server) handle(ctx context.Context, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return s.initialize(req.Params), nil
	case "ping":
		return struct{}{}, nil
	case "tools/list":
		return s.list(), nil
	case "tools/call":
		return s.call(ctx, req.Params)
	}
	return nil, &rpcError{codeMethodNotFound, "unknown method: " + req.Method}
}

// 아는 버전이면 그대로, 모르면 최신 버전으로 답한다. 클라이언트가 그 버전을 쓸 수 없으면 연결을 끊는다 (MCP 버전 협상).
func (s *Server) initialize(params json.RawMessage) initializeResult {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	json.Unmarshal(params, &p)
	version := versions[0]
	if slices.Contains(versions, p.ProtocolVersion) {
		version = p.ProtocolVersion
	}
	return initializeResult{ProtocolVersion: version, ServerInfo: s.info, Instructions: s.instructions}
}

func (s *Server) list() map[string][]toolDescriptor {
	descriptors := make([]toolDescriptor, len(s.tools))
	for i, t := range s.tools {
		descriptors[i] = toolDescriptor{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
	}
	return map[string][]toolDescriptor{"tools": descriptors}
}

func (s *Server) call(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{codeInvalidParams, err.Error()}
	}
	for _, t := range s.tools {
		if t.Name != p.Name {
			continue
		}
		if len(p.Arguments) == 0 {
			p.Arguments = json.RawMessage("{}")
		}
		result, err := t.Call(ctx, p.Arguments)
		if err != nil {
			return callResult{Content: Result{Text: err.Error()}.content(), IsError: true}, nil
		}
		return callResult{Content: result.content()}, nil
	}
	return nil, &rpcError{codeInvalidParams, "unknown tool: " + p.Name}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func RequireToken(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "a valid token is required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
