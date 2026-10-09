package mcp

import (
	"cmp"
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

const (
	codeParse          = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	latestVersion      = "2025-06-18"
)

type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Call        func(ctx context.Context, args json.RawMessage) (string, error)
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
	Type string `json:"type"`
	Text string `json:"text"`
}

type callResult struct {
	Content []content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Streamable HTTP 의 응답 하나짜리 형태만 쓴다. 서버가 먼저 보낼 메시지가 없어서 SSE 스트림은 열지 않는다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "POST 만 받는다", http.StatusMethodNotAllowed)
		return
	}
	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: &rpcError{codeParse, err.Error()}})
		return
	}
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	result, rpcErr := s.handle(r.Context(), req)
	writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr})
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
	return nil, &rpcError{codeMethodNotFound, "모르는 메서드: " + req.Method}
}

// 도구 호출만 쓰므로 2024-11-05 이후 버전은 주고받는 형식이 같다. 클라이언트가 요청한 버전을 그대로 쓴다.
func (s *Server) initialize(params json.RawMessage) initializeResult {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	json.Unmarshal(params, &p)
	return initializeResult{ProtocolVersion: cmp.Or(p.ProtocolVersion, latestVersion), ServerInfo: s.info, Instructions: s.instructions}
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
		text, err := t.Call(ctx, p.Arguments)
		if err != nil {
			return callResult{Content: []content{{"text", err.Error()}}, IsError: true}, nil
		}
		return callResult{Content: []content{{"text", text}}}, nil
	}
	return nil, &rpcError{codeInvalidParams, "없는 도구: " + p.Name}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func RequireToken(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "토큰이 필요하다", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
