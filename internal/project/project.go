package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	ErrNotEditable = errors.New("not an editable file")
	ErrChanged     = errors.New("the file changed since it was read")
)

var designSystem = []string{"DESIGN.md", "design.css"}

const stateDir = ".hunsu"

type Project struct {
	dir  string
	root *os.Root
}

type Listing struct {
	DesignSystem []Entry  `json:"designSystem"`
	Pages        []string `json:"pages"`
	Assets       []string `json:"assets"`
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

// StatePath 는 hunsu 상태 폴더(.hunsu) 안의 절대 경로다.
func (p *Project) StatePath(elem ...string) string {
	return filepath.Join(append([]string{p.dir, stateDir}, elem...)...)
}

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
	listing.Assets, err = p.assets()
	return listing, err
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

// Tag 는 내용을 나타내는 값이다. 읽은 뒤로 파일이 바뀌었는지 비교하는 데 쓴다.
func Tag(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// WriteIf 는 파일 내용이 아직 tag 일 때만 쓴다. hunsu 를 거치지 않는 외부 에이전트의 변경을 덮어쓰지 않게 한다.
func (p *Project) WriteIf(name string, data []byte, tag string) error {
	current, err := p.Read(name)
	if errors.Is(err, fs.ErrNotExist) || err == nil && Tag(current) != tag {
		return ErrChanged
	}
	if err != nil {
		return err
	}
	return p.Write(name, data)
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
