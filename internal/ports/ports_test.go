package ports_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"hunsu/internal/ports"
)

type result struct {
	port     int
	assigned map[string]int
	failed   bool
}

func TestListen(t *testing.T) {
	const dir, other = "/designs/a", "/designs/b"
	tests := []struct {
		name     string
		assigned map[string]int
		busy     []int
		want     result
	}{
		{"처음 연 폴더는 첫 포트를 쓰고 기억한다", nil, nil, result{0, map[string]int{dir: 0}, false}},
		{"기억해 둔 포트를 다시 쓴다", map[string]int{dir: 2}, nil, result{2, map[string]int{dir: 2}, false}},
		{"다른 폴더가 기억해 둔 포트는 건너뛴다", map[string]int{other: 0}, nil, result{1, map[string]int{other: 0, dir: 1}, false}},
		{"다른 프로그램이 쓰는 포트는 건너뛴다", nil, []int{0}, result{1, map[string]int{dir: 1}, false}},
		{"기억해 둔 포트를 다른 프로그램이 쓰면 다른 포트로 옮기지 않고 실패한다", map[string]int{dir: 0}, []int{0}, result{0, map[string]int{dir: 0}, true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := freePorts(t, 3)
			registry := filepath.Join(t.TempDir(), "hunsu", "ports.json")
			if tt.assigned != nil {
				writeRegistry(t, registry, shift(tt.assigned, first))
			}
			for _, offset := range tt.busy {
				occupy(t, first+offset)
			}
			got := result{failed: true}
			ln, err := ports.Listen(registry, dir, first)
			if err == nil {
				defer ln.Close()
				got = result{port: ln.Addr().(*net.TCPAddr).Port - first, failed: false}
			}
			got.assigned = shift(readRegistry(t, registry), -first)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Listen() = %+v (error %v), want %+v", got, err, tt.want)
			}
		})
	}
}

// freePorts 는 n 개가 연달아 비어 있는 포트의 첫 번호를 찾는다.
func freePorts(t *testing.T, n int) int {
	t.Helper()
	for first := 41000; first < 60000; first += 100 {
		free := true
		for port := first; port < first+n && free; port++ {
			ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			free = err == nil
			if free {
				ln.Close()
			}
		}
		if free {
			return first
		}
	}
	t.Fatal("비어 있는 포트가 없다")
	return 0
}

func occupy(t *testing.T, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
}

func shift(assigned map[string]int, by int) map[string]int {
	out := map[string]int{}
	for dir, port := range assigned {
		out[dir] = port + by
	}
	return out
}

func writeRegistry(t *testing.T, path string, assigned map[string]int) {
	t.Helper()
	data, _ := json.Marshal(assigned)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readRegistry(t *testing.T, path string) map[string]int {
	t.Helper()
	assigned := map[string]int{}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return assigned
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &assigned); err != nil {
		t.Fatal(err)
	}
	return assigned
}
