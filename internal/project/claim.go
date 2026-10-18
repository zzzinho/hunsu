package project

import (
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
)

var ErrRunning = errors.New("hunsu is already running for this folder")

const lockFile = "lock"

// Claim 은 폴더 하나를 hunsu 하나만 맡게 한다. 두 프로세스가 같은 .hunsu/ 를 쓰면 되돌리기·버전이 섞이고 실행 잠금이 갈린다.
// 운영체제 파일 잠금이라 프로세스가 비정상으로 끝나도 풀린다.
type Claim struct {
	file *os.File
}

// Claim 은 이 폴더를 맡는다. 다른 hunsu 가 맡고 있으면 그 hunsu 가 알린 주소와 ErrRunning 을 돌려준다.
// 주소를 알리기 전이면 주소는 비어 있다.
func (p *Project) Claim() (*Claim, string, error) {
	state, err := p.State()
	if err != nil {
		return nil, "", err
	}
	defer state.Close()
	f, err := state.OpenFile(lockFile, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, "", err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		url, _ := io.ReadAll(f)
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, strings.TrimSpace(string(url)), ErrRunning
		}
		return nil, "", err
	}
	return &Claim{file: f}, "", nil
}

// Announce 는 이 폴더를 맡은 hunsu 의 주소를 남겨, 같은 폴더로 다시 실행하면 그 화면을 열게 한다.
func (c *Claim) Announce(url string) error {
	if err := c.file.Truncate(0); err != nil {
		return err
	}
	_, err := c.file.WriteAt([]byte(url), 0)
	return err
}

func (c *Claim) Close() error { return c.file.Close() }
