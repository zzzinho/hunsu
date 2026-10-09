package project

import _ "embed"

// 브라우저 채팅(claude -p)과 Claude Code(MCP) 가 같은 디자인 규칙을 따르도록 한 곳에 둔다.
//
//go:embed rules.md
var Rules string
