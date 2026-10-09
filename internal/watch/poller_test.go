package watch_test

import (
	"maps"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"hunsu/internal/watch"
)

func file(modSec int64, data string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(data), ModTime: time.Unix(modSec, 0)}
}

func TestPoller_Poll(t *testing.T) {
	tests := []struct {
		name          string
		before, after fstest.MapFS
		want          []string
	}{
		{"수정 시각이 바뀐 파일을 알린다", fstest.MapFS{"a": file(1, "x")}, fstest.MapFS{"a": file(2, "x")}, []string{"a"}},
		{"수정 시각이 같아도 크기가 바뀌면 알린다", fstest.MapFS{"a": file(1, "x")}, fstest.MapFS{"a": file(1, "xy")}, []string{"a"}},
		{"바뀌지 않은 파일은 알리지 않는다", fstest.MapFS{"a": file(1, "x")}, fstest.MapFS{"a": file(1, "x")}, nil},
		{"새로 생긴 파일을 알린다", fstest.MapFS{}, fstest.MapFS{"a": file(1, "x")}, []string{"a"}},
		{"삭제된 파일을 알린다", fstest.MapFS{"a": file(1, "x")}, fstest.MapFS{}, []string{"a"}},
		{"감시하지 않는 파일은 알리지 않는다", fstest.MapFS{"b": file(1, "x")}, fstest.MapFS{"b": file(2, "x")}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{}
			maps.Copy(fsys, tt.before)
			p := watch.NewPoller(fsys, []string{"a"})
			clear(fsys)
			maps.Copy(fsys, tt.after)
			if got := p.Poll(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Poll() = %v, want %v", got, tt.want)
			}
			if again := p.Poll(); again != nil {
				t.Errorf("두 번째 Poll() = %v, 한 번 알린 변경은 다시 알리지 않아야 한다", again)
			}
		})
	}
}
