package chat

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"hunsu/internal/project"
)

// body 처럼 큰 요소를 고르면 페이지 전체가 들어온다. 위치는 선택자로 알 수 있으니 HTML 은 앞부분만 보낸다.
const selectionLimit = 2000

//go:embed instructions.md
var instructions string

// 바뀌지 않는 지시문만 둔다. 바뀌는 내용이 앞에 오면 그 뒤의 대화 기록 전체가 프롬프트 캐시에서 빠진다 (PLAN.md 11).
func systemPrompt() string { return instructions + project.Rules }

type designSection struct {
	files map[string][]byte
	whole bool
}

// 대화의 첫 요청에는 전체를, 이후에는 사용자가 바꾼 파일만 보낸다. LLM 이 고친 내용은 LLM 이 이미 안다.
func designUpdate(current map[string][]byte, edited []string, first bool) designSection {
	files := map[string][]byte{}
	for _, name := range project.DesignSystemFiles() {
		if data, ok := current[name]; ok && (first || slices.Contains(edited, name)) {
			files[name] = data
		}
	}
	return designSection{files: files, whole: first}
}

func userPrompt(t Turn, edited []string, design designSection, assets []string) string {
	var b strings.Builder
	if t.Page != "" {
		fmt.Fprintf(&b, "[현재 페이지] %s\n", t.Page)
	}
	if len(edited) > 0 {
		fmt.Fprintf(&b, "[알림] 마지막 응답 이후 사용자가 직접 고친 파일: %s. 고치기 전에 다시 읽어라.\n", strings.Join(edited, ", "))
	}
	design.write(&b)
	if len(assets) > 0 {
		fmt.Fprintf(&b, "[에셋] %s\n", strings.Join(assets, ", "))
	}
	if t.Selection != nil {
		fmt.Fprintf(&b, "[선택한 요소] %s\n%s\n", t.Selection.Selector, truncate(t.Selection.HTML, selectionLimit))
	}
	fmt.Fprintf(&b, "[요청] %s", t.Prompt)
	return b.String()
}

func (d designSection) write(b *strings.Builder) {
	if len(d.files) == 0 {
		if d.whole {
			b.WriteString("[디자인 시스템] 아직 없다\n")
		}
		return
	}
	b.WriteString("[디자인 시스템]\n")
	for _, name := range project.DesignSystemFiles() {
		if data, ok := d.files[name]; ok {
			fmt.Fprintf(b, "<%s>\n%s\n</%s>\n", name, data, name)
		}
	}
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}
