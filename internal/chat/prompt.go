package chat

import (
	_ "embed"
	"fmt"
	"strings"

	"hunsu/internal/project"
)

// body 처럼 큰 요소를 고르면 페이지 전체가 들어온다. 위치는 선택자로 알 수 있으니 HTML 은 앞부분만 보낸다.
const selectionLimit = 2000

//go:embed instructions.md
var instructions string

func userPrompt(t Turn, edited []string) string {
	var b strings.Builder
	if t.Page != "" {
		fmt.Fprintf(&b, "[현재 페이지] %s\n", t.Page)
	}
	if len(edited) > 0 {
		fmt.Fprintf(&b, "[알림] 마지막 응답 이후 사용자가 직접 고친 파일: %s. 고치기 전에 다시 읽어라.\n", strings.Join(edited, ", "))
	}
	if t.Selection != nil {
		fmt.Fprintf(&b, "[선택한 요소] %s\n%s\n", t.Selection.Selector, truncate(t.Selection.HTML, selectionLimit))
	}
	fmt.Fprintf(&b, "[요청] %s", t.Prompt)
	return b.String()
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}

func systemPrompt(files map[string][]byte) string {
	var b strings.Builder
	b.WriteString(instructions)
	for _, name := range project.DesignSystemFiles() {
		if data, ok := files[name]; ok {
			fmt.Fprintf(&b, "\n<%s>\n%s\n</%s>\n", name, data, name)
		}
	}
	if b.Len() == len(instructions) {
		b.WriteString("\n아직 디자인 시스템이 없다.")
	}
	return b.String()
}
