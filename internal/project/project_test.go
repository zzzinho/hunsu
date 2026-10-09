package project_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
)

const outsideLink = "outside-link.html"

var noDesignSystem = []project.Entry{{Path: "DESIGN.md"}, {Path: "design.css"}}

func newProject(t *testing.T, files ...string) *project.Project {
	t.Helper()
	contents := map[string]string{}
	for _, f := range files {
		contents[f] = f
	}
	delete(contents, outsideLink)
	p, dir := projecttest.New(t, contents)
	if slices.Contains(files, outsideLink) {
		must(t, os.WriteFile(filepath.Join(dir, "..", "outside.html"), []byte("outside"), 0o644))
		must(t, os.Symlink("../outside.html", filepath.Join(dir, outsideLink)))
	}
	return p
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditable(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"루트 폴더의 html 파일은 페이지로 편집한다", "landing.html", true},
		{"DESIGN.md 는 디자인 시스템 파일로 편집한다", "DESIGN.md", true},
		{"design.css 는 디자인 시스템 파일로 편집한다", "design.css", true},
		{"하위 폴더의 파일은 편집하지 않는다", "pages/about.html", false},
		{"폴더 밖 경로는 편집하지 않는다", "../landing.html", false},
		{"숨김 파일은 편집하지 않는다", ".draft.html", false},
		{"그 밖의 파일은 편집하지 않는다", "notes.txt", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := project.Editable(tt.path); got != tt.want {
				t.Errorf("Editable(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestProject_List(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  project.Listing
	}{
		{"편집 대상인 html 파일을 이름순으로 페이지에 나열한다",
			[]string{"b.html", "a.html", "notes.txt"},
			project.Listing{DesignSystem: noDesignSystem, Pages: []string{"a.html", "b.html"}}},
		{"심볼릭 링크는 페이지에 나열하지 않는다",
			[]string{outsideLink},
			project.Listing{DesignSystem: noDesignSystem, Pages: []string{}}},
		{"디자인 시스템 파일은 없어도 나열하고 존재 여부를 표시한다",
			[]string{"design.css"},
			project.Listing{DesignSystem: []project.Entry{{Path: "DESIGN.md"}, {Path: "design.css", Exists: true}}, Pages: []string{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newProject(t, tt.files...).List()
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List() = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestProject_Read(t *testing.T) {
	p := newProject(t, "page.html", "notes.txt", outsideLink)
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr error
	}{
		{"편집 대상 파일의 내용을 읽는다", "page.html", "page.html", nil},
		{"편집 대상이 아닌 파일은 읽지 않는다", "notes.txt", "", project.ErrNotEditable},
		{"심볼릭 링크는 읽지 않는다", outsideLink, "", project.ErrNotEditable},
		{"없는 파일은 없다고 알린다", "none.html", "", fs.ErrNotExist},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.Read(tt.path)
			if string(got) != tt.want || !errors.Is(err, tt.wantErr) {
				t.Errorf("Read(%q) = %q, %v; want %q, %v", tt.path, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestProject_Write(t *testing.T) {
	p := newProject(t, "page.html", "notes.txt", outsideLink)
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{"편집 대상 파일에 내용을 저장한다", "page.html", nil},
		{"편집 대상이 아닌 파일에는 저장하지 않는다", "notes.txt", project.ErrNotEditable},
		{"심볼릭 링크에는 저장하지 않는다", outsideLink, project.ErrNotEditable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Write(tt.path, []byte("saved"))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Write(%q) error = %v, want %v", tt.path, err, tt.wantErr)
			}
			if got, _ := p.Read(tt.path); err == nil && string(got) != "saved" {
				t.Errorf("Write(%q) 후 내용 = %q, want %q", tt.path, got, "saved")
			}
		})
	}
}

func TestProject_Create(t *testing.T) {
	p := newProject(t, "page.html")
	tests := []struct {
		name         string
		path         string
		wantContains string
		wantErr      error
	}{
		{"새 페이지는 design.css 를 링크한 html 로 만든다", "new.html", `<link rel="stylesheet" href="design.css">`, nil},
		{"design.css 는 CSS 변수 템플릿으로 만든다", "design.css", ":root {", nil},
		{"DESIGN.md 는 디자인 규칙 템플릿으로 만든다", "DESIGN.md", "# 디자인 규칙", nil},
		{"이미 있는 파일은 덮어쓰지 않는다", "page.html", "page.html", fs.ErrExist},
		{"편집 대상이 아닌 파일은 만들지 않는다", "notes.txt", "", project.ErrNotEditable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Create(tt.path)
			got, _ := p.Read(tt.path)
			if !errors.Is(err, tt.wantErr) || !strings.Contains(string(got), tt.wantContains) {
				t.Errorf("Create(%q) error = %v, 내용 = %q; want %v, %q 포함", tt.path, err, got, tt.wantErr, tt.wantContains)
			}
		})
	}
}

func TestListing_Files(t *testing.T) {
	tests := []struct {
		name    string
		listing project.Listing
		want    []string
	}{
		{"존재하는 디자인 시스템 파일과 페이지를 모두 돌려준다",
			project.Listing{DesignSystem: []project.Entry{{Path: "DESIGN.md"}, {Path: "design.css", Exists: true}}, Pages: []string{"a.html"}},
			[]string{"design.css", "a.html"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.listing.Files(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Files() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProject_Remove(t *testing.T) {
	p := newProject(t, "page.html", "notes.txt")
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{"편집 대상 파일을 지운다", "page.html", nil},
		{"편집 대상이 아닌 파일은 지우지 않는다", "notes.txt", project.ErrNotEditable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Remove(tt.path)
			if _, readErr := p.Read(tt.path); !errors.Is(err, tt.wantErr) || (err == nil && !errors.Is(readErr, fs.ErrNotExist)) {
				t.Errorf("Remove(%q) error = %v, 이후 Read error = %v; want %v", tt.path, err, readErr, tt.wantErr)
			}
		})
	}
}
