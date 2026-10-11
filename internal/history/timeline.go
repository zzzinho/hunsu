package history

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"time"

	"hunsu/internal/project"
)

var ErrNoVersion = errors.New("없는 버전")

const (
	versionsDir  = "versions"
	keepVersions = 100
	filesDir     = "files"
	metaFile     = "meta.json"
	thumbFile    = "thumb.png"
)

// Thumbnailer 는 페이지의 썸네일 PNG 를 만든다.
type Thumbnailer func(page string) ([]byte, error)

type Version struct {
	ID    int       `json:"id"`
	Time  time.Time `json:"time"`
	Label string    `json:"label"`
	Page  string    `json:"page"`
	Thumb bool      `json:"thumb"`
	Hash  string    `json:"hash"`
}

// Timeline 은 디자인 파일 전체의 저장본(버전)을 시간순으로 남긴다. 에셋은 넣지 않는다.
type Timeline struct {
	project *project.Project
	state   *os.Root
	thumb   Thumbnailer
}

// thumb 가 nil 이면 썸네일 없이 남긴다.
func NewTimeline(p *project.Project, thumb Thumbnailer) (*Timeline, error) {
	state, err := p.State()
	if err != nil {
		return nil, err
	}
	return &Timeline{project: p, state: state, thumb: thumb}, nil
}

// Keep 은 지금 상태를 버전으로 남긴다. 디자인 파일이 없거나 직전 버전과 같으면 남기지 않고 false 다.
func (t *Timeline) Keep(label, page string) (bool, error) {
	listing, err := t.project.List()
	if err != nil || len(listing.Files()) == 0 {
		return false, err
	}
	names, err := numbered(t.state, versionsDir)
	if err != nil {
		return false, err
	}
	id := next(names)
	dir := path.Join(versionsDir, id)
	hash, err := capture(t.project, t.state, path.Join(dir, filesDir))
	if err != nil {
		return false, err
	}
	if len(names) > 0 {
		if latest, err := t.meta(names[len(names)-1]); err == nil && latest.Hash == hash {
			return false, t.state.RemoveAll(dir)
		}
	}
	number, _ := strconv.Atoi(id)
	v := Version{ID: number, Time: time.Now(), Label: label, Page: page, Hash: hash}
	if t.thumb != nil && page != "" {
		// 썸네일은 보기 좋으라고 두는 것이다. 만들지 못해도 버전은 남긴다.
		if png, err := t.thumb(page); err == nil {
			v.Thumb = t.state.WriteFile(path.Join(dir, thumbFile), png, 0o644) == nil
		}
	}
	data, err := json.Marshal(v)
	if err != nil {
		return false, err
	}
	if err := t.state.WriteFile(path.Join(dir, metaFile), data, 0o644); err != nil {
		return false, err
	}
	return true, prune(t.state, versionsDir, append(names, id), keepVersions)
}

// List 는 최신 버전부터 돌려준다.
func (t *Timeline) List() ([]Version, error) {
	names, err := numbered(t.state, versionsDir)
	if err != nil {
		return nil, err
	}
	versions := []Version{}
	for _, n := range slices.Backward(names) {
		if v, err := t.meta(n); err == nil {
			versions = append(versions, v)
		}
	}
	return versions, nil
}

func (t *Timeline) Version(id int) (Version, error) {
	v, err := t.meta(name(id))
	if errors.Is(err, fs.ErrNotExist) {
		return Version{}, ErrNoVersion
	}
	return v, err
}

func (t *Timeline) Restore(id int) (Version, error) {
	v, err := t.Version(id)
	if err != nil {
		return Version{}, err
	}
	return v, restore(t.project, t.state, path.Join(versionsDir, name(id), filesDir))
}

func (t *Timeline) Thumbnail(id int) ([]byte, error) {
	return t.state.ReadFile(path.Join(versionsDir, name(id), thumbFile))
}

func (t *Timeline) meta(id string) (Version, error) {
	data, err := t.state.ReadFile(path.Join(versionsDir, id, metaFile))
	if err != nil {
		return Version{}, err
	}
	var v Version
	err = json.Unmarshal(data, &v)
	return v, err
}
