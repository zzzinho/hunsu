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

type promptInput struct {
	turn    Turn
	edited  []string
	design  designSection
	assets  []string
	variant string
}

func userPrompt(in promptInput) string {
	var b strings.Builder
	if in.turn.Page != "" {
		fmt.Fprintf(&b, "[Current page] %s\n", in.turn.Page)
	}
	if len(in.edited) > 0 {
		fmt.Fprintf(&b, "[Notice] Files the user edited since your last reply: %s. Re-read them before editing.\n", strings.Join(in.edited, ", "))
	}
	in.design.write(&b)
	if len(in.assets) > 0 {
		fmt.Fprintf(&b, "[Assets] %s\n", strings.Join(in.assets, ", "))
	}
	if s := in.turn.Selection; s != nil {
		fmt.Fprintf(&b, "[Selected element] %s\n%s\n", s.Selector, truncate(s.HTML, selectionLimit))
	}
	if in.variant != "" {
		fmt.Fprintf(&b, "[Variant] %s\n", in.variant)
	}
	fmt.Fprintf(&b, "[Request] %s", in.turn.Prompt)
	return b.String()
}

func (d designSection) write(b *strings.Builder) {
	if len(d.files) == 0 {
		if d.whole {
			b.WriteString("[Design system] none yet\n")
		}
		return
	}
	b.WriteString("[Design system]\n")
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
