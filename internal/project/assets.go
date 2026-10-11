package project

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const (
	assetDir     = "assets"
	MaxAssetSize = 10 << 20
)

var (
	ErrNotAsset = errors.New("에셋은 이미지(png·jpg·gif·webp·avif·svg)와 폰트(woff·woff2·ttf·otf)만")
	ErrTooLarge = errors.New("에셋은 10MB 까지")
)

var assetTypes = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg", ".woff", ".woff2", ".ttf", ".otf"}

func isAsset(name string) bool {
	return filepath.Base(name) == name && !strings.HasPrefix(name, ".") &&
		slices.Contains(assetTypes, strings.ToLower(filepath.Ext(name)))
}

func (p *Project) AddAsset(name string, r io.Reader) error {
	if !isAsset(name) {
		return ErrNotAsset
	}
	if err := p.root.MkdirAll(assetDir, 0o755); err != nil {
		return err
	}
	target := path.Join(assetDir, name)
	f, err := p.root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(r, MaxAssetSize+1))
	if err == nil && n > MaxAssetSize {
		err = ErrTooLarge
	}
	if err = errors.Join(err, f.Close()); err != nil {
		return errors.Join(err, p.root.Remove(target))
	}
	return nil
}

func (p *Project) assets() ([]string, error) {
	entries, err := fs.ReadDir(p.root.FS(), assetDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	names := []string{}
	for _, e := range entries {
		if isAsset(e.Name()) && e.Type().IsRegular() {
			names = append(names, path.Join(assetDir, e.Name()))
		}
	}
	return names, err
}
