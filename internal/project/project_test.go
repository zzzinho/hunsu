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

func TestIsDesignFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"페이지는 디자인 파일이다", "index.html", true},
		{"디자인 시스템은 디자인 파일이다", "design.css", true},
		{"assets 폴더의 이미지·폰트는 디자인 파일이다", "assets/logo.png", true},
		{"hunsu 의 상태 폴더는 디자인 파일이 아니다", ".hunsu/session", false},
		{"그 밖의 파일은 디자인 파일이 아니다", "notes.txt", false},
		{"assets 폴더의 이미지·폰트가 아닌 파일은 디자인 파일이 아니다", "assets/notes.txt", false},
		{"폴더는 디자인 파일이 아니다", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := project.IsDesignFile(tt.path); got != tt.want {
				t.Errorf("IsDesignFile(%q) = %v, want %v", tt.path, got, tt.want)
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
			project.Listing{DesignSystem: noDesignSystem, Pages: []string{"a.html", "b.html"}, Assets: []string{}}},
		{"심볼릭 링크는 페이지에 나열하지 않는다",
			[]string{outsideLink},
			project.Listing{DesignSystem: noDesignSystem, Pages: []string{}, Assets: []string{}}},
		{"assets 폴더의 이미지·폰트를 이름순으로 에셋에 나열한다",
			[]string{"assets/logo.png", "assets/Body.WOFF2", "assets/notes.txt", "assets/.hidden.png"},
			project.Listing{DesignSystem: noDesignSystem, Pages: []string{}, Assets: []string{"assets/Body.WOFF2", "assets/logo.png"}}},
		{"디자인 시스템 파일은 없어도 나열하고 존재 여부를 표시한다",
			[]string{"design.css"},
			project.Listing{DesignSystem: []project.Entry{{Path: "DESIGN.md"}, {Path: "design.css", Exists: true}}, Pages: []string{}, Assets: []string{}}},
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

func TestProject_WriteIf(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		read    string
		wantErr error
		want    string
	}{
		{"읽은 뒤로 바뀌지 않았으면 저장한다", "page.html", "page.html", nil, "saved"},
		{"읽은 뒤로 다른 곳에서 바뀌었으면 저장하지 않는다", "page.html", "old", project.ErrChanged, "page.html"},
		{"읽은 뒤로 지워졌으면 다시 만들지 않는다", "gone.html", "gone.html", project.ErrChanged, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newProject(t, "page.html")
			err := p.WriteIf(tt.path, []byte("saved"), project.Tag([]byte(tt.read)))
			if got, _ := p.Read(tt.path); !errors.Is(err, tt.wantErr) || string(got) != tt.want {
				t.Errorf("WriteIf(%q) = %v, 내용 %q; want %v, %q", tt.path, err, got, tt.wantErr, tt.want)
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

func TestProject_AddAsset(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		size    int
		wantErr error
	}{
		{"이미지를 assets 폴더에 올린다", "logo.png", 10, nil},
		{"폰트를 assets 폴더에 올린다", "body.woff2", 10, nil},
		{"확장자의 대소문자는 가리지 않는다", "photo.JPG", 10, nil},
		{"10MB 까지 올린다", "big.png", project.MaxAssetSize, nil},
		{"10MB 를 넘으면 올리지 않는다", "huge.png", project.MaxAssetSize + 1, project.ErrTooLarge},
		{"이미지·폰트가 아니면 올리지 않는다", "script.js", 10, project.ErrNotAsset},
		{"하위 폴더 경로로는 올리지 않는다", "../logo.png", 10, project.ErrNotAsset},
		{"숨김 파일은 올리지 않는다", ".logo.png", 10, project.ErrNotAsset},
		{"같은 이름이 있으면 덮어쓰지 않는다", "taken.png", 10, fs.ErrExist},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newProject(t, "assets/taken.png")
			err := p.AddAsset(tt.file, strings.NewReader(strings.Repeat("x", tt.size)))
			saved, _ := fs.ReadFile(p.FS(), "assets/"+tt.file)
			uploaded := len(saved) == tt.size
			if !errors.Is(err, tt.wantErr) || uploaded != (tt.wantErr == nil) {
				t.Errorf("AddAsset(%q) error = %v, 올린 내용 저장 = %v; want %v", tt.file, err, uploaded, tt.wantErr)
			}
		})
	}
}

func TestAssetType(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
	}{
		{"이미지 에셋의 MIME 타입을 알려준다", "assets/logo.PNG", "image/png"},
		{"폰트 에셋의 MIME 타입을 알려준다", "assets/body.woff2", "font/woff2"},
		{"에셋이 아니면 빈 문자열이다", "assets/notes.txt", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := project.AssetType(tt.file); got != tt.want {
				t.Errorf("AssetType(%q) = %q, want %q", tt.file, got, tt.want)
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
