package history

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"strconv"

	"hunsu/internal/project"
)

var ErrNoVariant = errors.New("no such variant")

const variantsDir = "variants"

// Variants 는 비교안을 만들 디자인 파일 복사본이다. 안마다 .hunsu/variants/<번호>/ 에 둔다.
type Variants struct {
	project *project.Project
	state   *os.Root
}

func NewVariants(p *project.Project) (*Variants, error) {
	state, err := p.State()
	if err != nil {
		return nil, err
	}
	return &Variants{project: p, state: state}, nil
}

// Prepare 는 지난 비교안을 지우고 지금 디자인 파일을 n 개 복사한다. 복사본의 절대 경로를 돌려준다.
func (v *Variants) Prepare(n int) ([]string, error) {
	if err := v.Clear(); err != nil {
		return nil, err
	}
	dirs := make([]string, n)
	for i := range n {
		dir := path.Join(variantsDir, strconv.Itoa(i+1))
		if _, err := capture(v.project, v.state, dir); err != nil {
			return nil, err
		}
		dirs[i] = v.project.StatePath(dir)
	}
	return dirs, nil
}

func (v *Variants) Pages(id int) ([]string, error) {
	matches, err := fs.Glob(v.state.FS(), path.Join(variantsDir, strconv.Itoa(id), "*.html"))
	pages := make([]string, len(matches))
	for i, m := range matches {
		pages[i] = path.Base(m)
	}
	return pages, err
}

// FS 는 안 id 의 파일이다. 렌더해서 비교할 때 쓴다.
func (v *Variants) FS(id int) (fs.FS, error) {
	dir := path.Join(variantsDir, strconv.Itoa(id))
	if _, err := v.state.Stat(dir); err != nil {
		return nil, ErrNoVariant
	}
	return fs.Sub(v.state.FS(), dir)
}

// Apply 는 디자인 파일을 안 id 로 바꾼다. 안 속의 디자인 파일이 아닌 파일은 무시한다.
func (v *Variants) Apply(id int) error {
	dir := path.Join(variantsDir, strconv.Itoa(id))
	if _, err := v.state.Stat(dir); err != nil {
		return ErrNoVariant
	}
	return restore(v.project, v.state, dir)
}

func (v *Variants) Clear() error { return v.state.RemoveAll(variantsDir) }
