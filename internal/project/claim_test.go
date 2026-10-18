package project_test

import (
	"errors"
	"testing"

	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
)

type other int

const (
	noOther other = iota
	otherRunning
	otherEnded
)

func TestProject_Claim(t *testing.T) {
	const url = "http://127.0.0.1:7778/"
	tests := []struct {
		name        string
		other       other
		wantRunning string
		wantErr     error
	}{
		{"아무도 맡지 않은 폴더를 맡는다", noOther, "", nil},
		{"다른 hunsu 가 맡은 폴더는 맡지 않고 그 hunsu 의 주소를 알려준다", otherRunning, url, project.ErrRunning},
		{"먼저 맡았던 hunsu 가 끝났으면 맡는다", otherEnded, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, dir := projecttest.New(t, nil)
			if tt.other != noOther {
				c, _, err := first.Claim()
				must(t, err)
				must(t, c.Announce(url))
				t.Cleanup(func() { c.Close() })
				if tt.other == otherEnded {
					must(t, c.Close())
				}
			}
			second, err := project.Open(dir)
			must(t, err)
			t.Cleanup(func() { second.Close() })
			c, running, err := second.Claim()
			if c != nil {
				t.Cleanup(func() { c.Close() })
			}
			if running != tt.wantRunning || !errors.Is(err, tt.wantErr) {
				t.Errorf("Claim() = %q, %v; want %q, %v", running, err, tt.wantRunning, tt.wantErr)
			}
		})
	}
}
