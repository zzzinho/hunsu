package main

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	defaults := options{dir: ".", open: true}
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr error
	}{
		{"폴더를 생략하면 현재 디렉터리를 연다", nil, defaults, nil},
		{"폴더를 인자로 받는다", []string{"designs"}, options{dir: "designs", open: true}, nil},
		{"폴더 앞의 옵션을 읽는다", []string{"-port", "8888", "designs"}, options{dir: "designs", port: 8888, open: true}, nil},
		{"폴더 뒤의 옵션도 읽는다", []string{"designs", "-no-open", "-provider", "codex"}, options{dir: "designs", provider: "codex"}, nil},
		{"폴더를 둘 이상 주면 거부한다", []string{"a", "-no-open", "b"}, options{}, errTooManyFolders},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args, io.Discard)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("parseArgs(%q) error = %v, want %v", tt.args, err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseArgs(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

func TestUsage(t *testing.T) {
	t.Run("도움말에 폴더 인자를 보인다", func(t *testing.T) {
		var out strings.Builder
		parseArgs([]string{"-h"}, &out)
		if !strings.Contains(out.String(), "[folder]") {
			t.Errorf("-h output does not mention the folder argument:\n%s", out.String())
		}
	})
}
