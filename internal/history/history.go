package history

import (
	"errors"
	"os"
	"path"

	"hunsu/internal/project"
)

var ErrNothingToUndo = errors.New("되돌릴 것이 없다")

const (
	dir           = "history"
	keepSnapshots = 50
)

type History struct {
	project *project.Project
	state   *os.Root
}

func New(p *project.Project) (*History, error) {
	state, err := p.State()
	if err != nil {
		return nil, err
	}
	return &History{project: p, state: state}, nil
}

func (h *History) Save() error {
	names, err := numbered(h.state, dir)
	if err != nil {
		return err
	}
	snapshot := next(names)
	if _, err := capture(h.project, h.state, path.Join(dir, snapshot)); err != nil {
		return err
	}
	return prune(h.state, dir, append(names, snapshot), keepSnapshots)
}

func (h *History) Undo() error {
	names, err := numbered(h.state, dir)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return ErrNothingToUndo
	}
	snapshot := path.Join(dir, names[len(names)-1])
	if err := restore(h.project, h.state, snapshot); err != nil {
		return err
	}
	return h.state.RemoveAll(snapshot)
}
