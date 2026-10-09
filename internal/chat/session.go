package chat

import (
	"errors"
	"io/fs"
	"os"
)

const sessionFile = "session"

type session struct {
	state *os.Root
}

func (s session) load() string {
	data, _ := s.state.ReadFile(sessionFile)
	return string(data)
}

func (s session) save(id string) error {
	return s.state.WriteFile(sessionFile, []byte(id), 0o644)
}

func (s session) reset() error {
	if err := s.state.Remove(sessionFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
