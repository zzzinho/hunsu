package claude_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hunsu/internal/claude"
)

func TestCLI_Run(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		prompt  string
		want    string
		wantErr string
	}{
		{"claude 출력을 진행 이벤트로 바꿔 전달한다", "edit.sh", "", "start write write:a.html reply:고쳤습니다", ""},
		{"프롬프트를 표준 입력으로 넘긴다", "echo.sh", "-로 시작하는 요청", "reply:-로 시작하는 요청", ""},
		{"claude 가 실패하면 표준 에러를 담아 알린다", "fail.sh", "", "", "boom"},
		{"취소하면 하위 프로세스가 남아 있어도 곧바로 멈춘다", "hang.sh", "", "", "claude 실행 실패"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := filepath.Abs(filepath.Join("testdata", tt.script))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()
			var events []string
			start := time.Now()
			err = claude.CLI{Path: path}.Run(ctx, claude.Request{Dir: t.TempDir(), Prompt: tt.prompt},
				func(e claude.Event) { events = append(events, e.String()) })
			got := strings.Join(events, " ")
			if got != tt.want || !errorContains(err, tt.wantErr) || time.Since(start) > 3*time.Second {
				t.Errorf("Run() = %q, %v (%s 걸림); want %q, 에러 %q 포함", got, err, time.Since(start), tt.want, tt.wantErr)
			}
		})
	}
}

func errorContains(err error, want string) bool {
	if want == "" {
		return err == nil
	}
	return err != nil && strings.Contains(err.Error(), want)
}
