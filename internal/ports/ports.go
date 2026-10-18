// Package ports 는 디자인 폴더마다 고정 포트를 정해 기억한다.
// 폴더마다 MCP 클라이언트에 등록한 주소가 실행할 때마다 바뀌지 않게 한다.
package ports

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
)

// First 는 처음 연 폴더의 포트다. 다음 폴더부터 하나씩 올라간다.
const First = 7777

const tries = 100

var ErrNoPort = errors.New("no free port")

// Listen 은 dir 에 정해 둔 포트로 듣는다. 정해 둔 포트가 없으면 다른 폴더에 정해 두지 않은 포트 중
// first 부터 비어 있는 것을 골라 registry 파일에 기억한다.
// 정해 둔 포트를 다른 프로그램이 쓰고 있으면 옮기지 않고 실패한다. 옮기면 등록한 주소가 맞지 않게 된다.
func Listen(registry, dir string, first int) (net.Listener, error) {
	assigned, err := load(registry)
	if err != nil {
		return nil, err
	}
	if port, ok := assigned[dir]; ok {
		return listen(port)
	}
	taken := map[int]bool{}
	for _, port := range assigned {
		taken[port] = true
	}
	for port := first; port < first+tries; port++ {
		if taken[port] {
			continue
		}
		ln, err := listen(port)
		if err != nil {
			continue
		}
		assigned[dir] = port
		// ponytail: 두 폴더를 동시에 처음 열면 한쪽 기록이 덮일 수 있다. 다음 실행에서 다시 고른다.
		if err := save(registry, assigned); err != nil {
			ln.Close()
			return nil, err
		}
		return ln, nil
	}
	return nil, ErrNoPort
}

func listen(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
}

func load(registry string) (map[string]int, error) {
	assigned := map[string]int{}
	data, err := os.ReadFile(registry)
	if errors.Is(err, fs.ErrNotExist) {
		return assigned, nil
	}
	if err != nil {
		return nil, err
	}
	return assigned, json.Unmarshal(data, &assigned)
}

func save(registry string, assigned map[string]int) error {
	data, err := json.MarshalIndent(assigned, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(registry), 0o700); err != nil {
		return err
	}
	tmp := registry + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, registry)
}
