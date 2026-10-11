package export_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"reflect"
	"testing"

	"hunsu/internal/export"
	"hunsu/internal/project/projecttest"
)

func TestHTML(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		page    string
		want    string
		wantErr error
	}{
		{"design.css 링크를 그 내용을 담은 style 태그로 바꾼다",
			map[string]string{"a.html": `<link rel="stylesheet" href="design.css"><p>a</p>`, "design.css": "p{color:red}"},
			"a.html", "<style>\np{color:red}\n</style><p>a</p>", nil},
		{"에셋 경로를 data URI 로 바꾼다",
			map[string]string{"a.html": `<img src="assets/logo.png">`, "assets/logo.png": "PNG"},
			"a.html", `<img src="data:image/png;base64,UE5H">`, nil},
		{"디자인 시스템 CSS 안의 에셋도 data URI 로 바꾼다",
			map[string]string{"a.html": `<link href="design.css" rel="stylesheet">`, "design.css": "@font-face{src:url(assets/a.woff2)}", "assets/a.woff2": "F"},
			"a.html", "<style>\n@font-face{src:url(data:font/woff2;base64,Rg==)}\n</style>", nil},
		{"퍼센트 인코딩되거나 ./ 로 시작하는 에셋 경로도 찾는다",
			map[string]string{"a.html": `<img src="./assets/my%20logo.png">`, "assets/my logo.png": "PNG"},
			"a.html", `<img src="data:image/png;base64,UE5H">`, nil},
		{"이름에 공백·괄호가 있는 에셋도 바꾼다",
			map[string]string{"a.html": `<img src="assets/Screen Shot (1).png"><i style="background:url('assets/logo%20(1).png')"></i>`, "assets/Screen Shot (1).png": "PNG", "assets/logo (1).png": "PNG"},
			"a.html", `<img src="data:image/png;base64,UE5H"><i style="background:url('data:image/png;base64,UE5H')"></i>`, nil},
		{"다른 주소 안의 assets 경로와 본문 글자는 바꾸지 않는다",
			map[string]string{"a.html": `<img src="https://cdn.test/assets/logo.png"><p>assets/logo.png 를 올린다</p>`, "assets/logo.png": "PNG"},
			"a.html", `<img src="https://cdn.test/assets/logo.png"><p>assets/logo.png 를 올린다</p>`, nil},
		{"없는 에셋 경로는 그대로 둔다",
			map[string]string{"a.html": `<img src="assets/none.png">`},
			"a.html", `<img src="assets/none.png">`, nil},
		{"페이지가 아니면 내보내지 않는다",
			map[string]string{"design.css": "p{}"},
			"design.css", "", export.ErrNotPage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := projecttest.New(t, tt.files)
			got, err := export.HTML(p, tt.page)
			if string(got) != tt.want || !errors.Is(err, tt.wantErr) {
				t.Errorf("HTML(%q) = %q, %v; want %q, %v", tt.page, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestZIP(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"디자인 시스템, 페이지, 에셋을 담고 .hunsu 와 그 밖의 파일은 뺀다",
			map[string]string{"design.css": "c", "a.html": "a", "assets/logo.png": "l", "notes.txt": "n", ".hunsu/session": "s"},
			[]string{"design.css", "a.html", "assets/logo.png"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := projecttest.New(t, tt.files)
			var buf bytes.Buffer
			if err := export.ZIP(&buf, p); err != nil {
				t.Fatal(err)
			}
			r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, f := range r.File {
				got = append(got, f.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ZIP 항목 = %v, want %v", got, tt.want)
			}
		})
	}
}
