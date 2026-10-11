package chat

import (
	"errors"
	"io/fs"
	"os"
)

// 대화 ID 는 provider 마다 형식이 달라 provider 별 파일에 둔다.
type session struct {
	state *os.Root
	file  string
}

func newSession(state *os.Root, provider string) session {
	return session{state: state, file: "session-" + provider}
}

func (s session) load() string {
	data, _ := s.state.ReadFile(s.file)
	return string(data)
}

func (s session) save(id string) error {
	return s.state.WriteFile(s.file, []byte(id), 0o644)
}

func (s session) reset() error {
	if err := s.state.Remove(s.file); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
