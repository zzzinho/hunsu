package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var ErrNotEditable = errors.New("편집 대상이 아닌 파일")

var designSystem = []string{"DESIGN.md", "design.css"}

const stateDir = ".hunsu"

type Project struct {
	dir  string
	root *os.Root
}

type Listing struct {
	DesignSystem []Entry  `json:"designSystem"`
	Pages        []string `json:"pages"`
}

type Entry struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

func Open(dir string) (*Project, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	return &Project{dir: abs, root: root}, nil
}

func (p *Project) Close() error { return p.root.Close() }

func (p *Project) FS() fs.FS { return p.root.FS() }

func (p *Project) Dir() string { return p.dir }

func (p *Project) State() (*os.Root, error) {
	if err := p.root.MkdirAll(stateDir, 0o755); err != nil {
		return nil, err
	}
	return p.root.OpenRoot(stateDir)
}

func (l Listing) Files() []string {
	var names []string
	for _, e := range l.DesignSystem {
		if e.Exists {
			names = append(names, e.Path)
		}
	}
	return append(names, l.Pages...)
}

func DesignSystemFiles() []string { return slices.Clone(designSystem) }

func Editable(name string) bool {
	return slices.Contains(designSystem, name) || isPage(name)
}

func isPage(name string) bool {
	return filepath.Base(name) == name && !strings.HasPrefix(name, ".") && filepath.Ext(name) == ".html"
}

func (p *Project) List() (Listing, error) {
	entries, err := fs.ReadDir(p.root.FS(), ".")
	if err != nil {
		return Listing{}, err
	}
	listing := Listing{Pages: []string{}}
	for _, name := range designSystem {
		listing.DesignSystem = append(listing.DesignSystem, Entry{Path: name, Exists: p.editableFile(name) == nil})
	}
	for _, e := range entries {
		if isPage(e.Name()) && p.editableFile(e.Name()) == nil {
			listing.Pages = append(listing.Pages, e.Name())
		}
	}
	return listing, nil
}

func (p *Project) Read(name string) ([]byte, error) {
	if err := p.editableFile(name); err != nil {
		return nil, err
	}
	return p.root.ReadFile(name)
}

func (p *Project) Write(name string, data []byte) error {
	if err := p.editableFile(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	// 렌더 화면이나 LLM 이 반쯤 쓰인 파일을 읽지 않도록 임시 파일에 쓴 뒤 이름을 바꾼다.
	tmp := "." + name + ".tmp"
	if err := p.root.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return p.root.Rename(tmp, name)
}

func (p *Project) Create(name string) error {
	if !Editable(name) {
		return ErrNotEditable
	}
	f, err := p.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(templateFor(name))
	return errors.Join(err, f.Close())
}

func (p *Project) Remove(name string) error {
	if err := p.editableFile(name); err != nil {
		return err
	}
	return p.root.Remove(name)
}

func (p *Project) editableFile(name string) error {
	if !Editable(name) {
		return ErrNotEditable
	}
	info, err := p.root.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return ErrNotEditable
	}
	return nil
}
