package history

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"

	"hunsu/internal/project"
)

// capture 는 지금 디자인 파일을 state 의 dir 에 복사하고, 내용을 나타내는 해시를 돌려준다.
func capture(p *project.Project, state *os.Root, dir string) (string, error) {
	listing, err := p.List()
	if err != nil {
		return "", err
	}
	if err := state.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, name := range listing.Files() {
		data, err := p.Read(name)
		if err != nil {
			return "", err
		}
		if err := state.WriteFile(path.Join(dir, name), data, 0o644); err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", name, len(data))
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// restore 는 dir 에 복사해 둔 파일로 디자인 파일을 되돌린다. 그 뒤에 생긴 파일은 지운다.
func restore(p *project.Project, state *os.Root, dir string) error {
	saved, err := fs.Glob(state.FS(), path.Join(dir, "*"))
	if err != nil {
		return err
	}
	savedNames := make([]string, len(saved))
	for i, s := range saved {
		savedNames[i] = path.Base(s)
	}
	listing, err := p.List()
	if err != nil {
		return err
	}
	for _, name := range listing.Files() {
		if !slices.Contains(savedNames, name) {
			if err := p.Remove(name); err != nil {
				return err
			}
		}
	}
	for _, name := range savedNames {
		data, err := state.ReadFile(path.Join(dir, name))
		if err != nil {
			return err
		}
		if err := p.Write(name, data); err != nil {
			return err
		}
	}
	return nil
}

// numbered 는 dir 아래 순번 이름(000001…)을 오래된 것부터 돌려준다.
func numbered(state *os.Root, dir string) ([]string, error) {
	entries, err := fs.ReadDir(state.FS(), dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, err
}

// 오래된 것을 지운 뒤에도 겹치지 않도록 마지막 순번 다음을 쓴다.
func next(names []string) string {
	last := 0
	if len(names) > 0 {
		last, _ = strconv.Atoi(names[len(names)-1])
	}
	return name(last + 1)
}

func name(id int) string { return fmt.Sprintf("%06d", id) }

func prune(state *os.Root, dir string, names []string, keep int) error {
	for len(names) > keep {
		if err := state.RemoveAll(path.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}
