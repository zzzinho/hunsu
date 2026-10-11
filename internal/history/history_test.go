package history_test

import (
	"errors"
	"fmt"
	"maps"
	"testing"

	"hunsu/internal/history"
	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
)

type files map[string]string

func TestHistory_Undo(t *testing.T) {
	tests := []struct {
		name    string
		states  []files
		undos   int
		want    files
		wantErr error
	}{
		{"고친 파일을 저장 시점의 내용으로 되돌린다", []files{{"a.html": "1"}, {"a.html": "2"}}, 1, files{"a.html": "1"}, nil},
		{"저장 뒤에 생긴 페이지는 지운다", []files{{"a.html": "1"}, {"a.html": "1", "b.html": "1"}}, 1, files{"a.html": "1"}, nil},
		{"저장 뒤에 지워진 파일은 다시 만든다", []files{{"a.html": "1", "design.css": "1"}, {"a.html": "1"}}, 1, files{"a.html": "1", "design.css": "1"}, nil},
		{"빈 프로젝트에서 저장했으면 생긴 파일을 모두 지운다", []files{{}, {"a.html": "1", "design.css": "1"}}, 1, files{}, nil},
		{"여러 번 되돌리면 더 이전 저장 시점으로 간다", []files{{"a.html": "1"}, {"a.html": "2"}, {"a.html": "3"}}, 2, files{"a.html": "1"}, nil},
		{"최근 50개 시점만 남기고 오래된 것부터 지운다", manyStates(53), 51, manyStates(53)[2], history.ErrNothingToUndo},
		{"오래된 시점을 지운 뒤에 저장해도 다른 시점을 덮어쓰지 않는다", manyStates(53), 2, manyStates(53)[50], nil},
		{"저장한 적이 없으면 되돌릴 것이 없다고 알린다", []files{{"a.html": "1"}}, 1, files{"a.html": "1"}, history.ErrNothingToUndo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := projecttest.New(t, nil)
			h, err := history.New(p)
			must(t, err)
			for i, state := range tt.states {
				setFiles(t, p, state)
				if i < len(tt.states)-1 {
					must(t, h.Save())
				}
			}
			for range tt.undos {
				err = h.Undo()
			}
			if got := readFiles(t, p); !errors.Is(err, tt.wantErr) || !maps.Equal(got, tt.want) {
				t.Errorf("Undo() 후 파일 = %v, error = %v; want %v, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func manyStates(n int) []files {
	states := make([]files, n)
	for i := range states {
		states[i] = files{"a.html": fmt.Sprint(i)}
	}
	return states
}

func setFiles(t *testing.T, p *project.Project, state files) {
	t.Helper()
	listing, err := p.List()
	must(t, err)
	for _, name := range listing.Files() {
		if _, keep := state[name]; !keep {
			must(t, p.Remove(name))
		}
	}
	for name, body := range state {
		must(t, p.Write(name, []byte(body)))
	}
}

func readFiles(t *testing.T, p *project.Project) files {
	t.Helper()
	listing, err := p.List()
	must(t, err)
	got := files{}
	for _, name := range listing.Files() {
		data, err := p.Read(name)
		must(t, err)
		got[name] = string(data)
	}
	return got
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
