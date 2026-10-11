package project

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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

var assetTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".avif": "image/avif", ".svg": "image/svg+xml",
	".woff": "font/woff", ".woff2": "font/woff2", ".ttf": "font/ttf", ".otf": "font/otf",
}

// AssetType 은 에셋의 MIME 타입이다. 에셋이 아니면 빈 문자열이다.
func AssetType(name string) string { return assetTypes[strings.ToLower(filepath.Ext(name))] }

func isAsset(name string) bool {
	return filepath.Base(name) == name && !strings.HasPrefix(name, ".") && AssetType(name) != ""
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
