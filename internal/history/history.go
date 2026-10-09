package history

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"

	"hunsu/internal/project"
)

var ErrNothingToUndo = errors.New("되돌릴 것이 없다")

const dir = "history"

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
	snapshots, err := h.snapshots()
	if err != nil {
		return err
	}
	snapshot := path.Join(dir, fmt.Sprintf("%06d", len(snapshots)+1))
	if err := h.state.MkdirAll(snapshot, 0o755); err != nil {
		return err
	}
	names, err := h.files()
	if err != nil {
		return err
	}
	for _, name := range names {
		data, err := h.project.Read(name)
		if err != nil {
			return err
		}
		if err := h.state.WriteFile(path.Join(snapshot, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (h *History) Undo() error {
	snapshots, err := h.snapshots()
	if err != nil {
		return err
	}
	if len(snapshots) == 0 {
		return ErrNothingToUndo
	}
	snapshot := path.Join(dir, snapshots[len(snapshots)-1])
	saved, err := fs.Glob(h.state.FS(), path.Join(snapshot, "*"))
	if err != nil {
		return err
	}
	savedNames := make([]string, len(saved))
	for i, s := range saved {
		savedNames[i] = path.Base(s)
	}
	current, err := h.files()
	if err != nil {
		return err
	}
	for _, name := range current {
		if !slices.Contains(savedNames, name) {
			if err := h.project.Remove(name); err != nil {
				return err
			}
		}
	}
	for _, name := range savedNames {
		data, err := h.state.ReadFile(path.Join(snapshot, name))
		if err != nil {
			return err
		}
		if err := h.project.Write(name, data); err != nil {
			return err
		}
	}
	return h.state.RemoveAll(snapshot)
}

func (h *History) snapshots() ([]string, error) {
	entries, err := fs.ReadDir(h.state.FS(), dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, err
}

func (h *History) files() ([]string, error) {
	listing, err := h.project.List()
	return listing.Files(), err
}
