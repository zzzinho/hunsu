package history_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"hunsu/internal/history"
	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
)

type variantsFixture struct {
	t        *testing.T
	project  *project.Project
	variants *history.Variants
	dirs     []string
}

func (f *variantsFixture) write(variant int, name, body string) {
	must(f.t, os.WriteFile(filepath.Join(f.dirs[variant-1], name), []byte(body), 0o644))
}

func TestVariants(t *testing.T) {
	tests := []struct {
		name string
		run  func(f *variantsFixture) string
		want string
	}{
		{"비교안 수만큼 지금 디자인 파일의 복사본을 만든다",
			func(f *variantsFixture) string {
				var out []string
				for _, dir := range f.dirs {
					data, err := os.ReadFile(filepath.Join(dir, "a.html"))
					must(f.t, err)
					out = append(out, filepath.Base(dir)+":"+string(data))
				}
				return fmt.Sprint(out)
			},
			"[1:원본 2:원본]"},
		{"다시 만들면 지난 비교안은 지운다",
			func(f *variantsFixture) string {
				_, err := f.variants.Prepare(1)
				must(f.t, err)
				_, statErr := os.Stat(f.dirs[1])
				return fmt.Sprint(errors.Is(statErr, os.ErrNotExist))
			},
			"true"},
		{"안의 페이지를 이름순으로 알려준다",
			func(f *variantsFixture) string {
				f.write(1, "b.html", "새 페이지")
				pages, err := f.variants.Pages(1)
				must(f.t, err)
				return fmt.Sprint(pages)
			},
			"[a.html b.html]"},
		{"안을 적용하면 그 안의 디자인 파일로 바꾸고, 그 안에 없는 페이지는 지운다",
			func(f *variantsFixture) string {
				f.write(2, "a.html", "안 2")
				setFiles(f.t, f.project, files{"a.html": "원본", "c.html": "나중에 생김"})
				must(f.t, f.variants.Apply(2))
				return fmt.Sprint(readFiles(f.t, f.project))
			},
			"map[a.html:안 2]"},
		{"안 속의 디자인 파일이 아닌 파일은 적용하지 않는다",
			func(f *variantsFixture) string {
				f.write(1, "notes.txt", "메모")
				must(f.t, f.variants.Apply(1))
				_, err := os.Stat(filepath.Join(f.project.Dir(), "notes.txt"))
				return fmt.Sprint(errors.Is(err, os.ErrNotExist))
			},
			"true"},
		{"없는 안은 적용하지 않는다",
			func(f *variantsFixture) string {
				return fmt.Sprint(errors.Is(f.variants.Apply(3), history.ErrNoVariant))
			},
			"true"},
		{"비교안을 치우면 복사본이 사라진다",
			func(f *variantsFixture) string {
				must(f.t, f.variants.Clear())
				return fmt.Sprint(errors.Is(f.variants.Apply(1), history.ErrNoVariant))
			},
			"true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := projecttest.New(t, map[string]string{"a.html": "원본"})
			v, err := history.NewVariants(p)
			must(t, err)
			dirs, err := v.Prepare(2)
			must(t, err)
			if got := tt.run(&variantsFixture{t: t, project: p, variants: v, dirs: dirs}); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
