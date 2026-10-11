package export

import (
	"archive/zip"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"hunsu/internal/project"
)

var ErrNotPage = errors.New("not a page")

var (
	stylesheetLink = regexp.MustCompile(`(?i)<link\b[^>]*\bhref\s*=\s*["']?(?:\./)?design\.css["']?[^>]*>`)
	assetRef       = regexp.MustCompile(`(?:\./)?assets/[^"'()\s<>&]+`)
)

// HTML 은 페이지 하나를 파일 하나로 만든다. design.css 와 에셋을 그 안에 넣는다.
func HTML(p *project.Project, page string) ([]byte, error) {
	listing, err := p.List()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(listing.Pages, page) {
		return nil, ErrNotPage
	}
	html, err := p.Read(page)
	if err != nil {
		return nil, err
	}
	// 에셋을 먼저 바꾼다. 넣은 base64 안에 우연히 생긴 "assets/" 를 다시 바꾸지 않게.
	html = inlineAssets(p, listing.Assets, html)
	css, err := p.Read("design.css")
	if errors.Is(err, fs.ErrNotExist) {
		return html, nil
	}
	if err != nil {
		return nil, err
	}
	style := slices.Concat([]byte("<style>\n"), inlineAssets(p, listing.Assets, css), []byte("\n</style>"))
	return stylesheetLink.ReplaceAllLiteral(html, style), nil
}

func inlineAssets(p *project.Project, assets []string, src []byte) []byte {
	return assetRef.ReplaceAllFunc(src, func(ref []byte) []byte {
		name, err := url.PathUnescape(strings.TrimPrefix(string(ref), "./"))
		if err != nil || !slices.Contains(assets, name) {
			return ref
		}
		data, err := fs.ReadFile(p.FS(), name)
		if err != nil {
			return ref
		}
		return []byte("data:" + project.AssetType(name) + ";base64," + base64.StdEncoding.EncodeToString(data))
	})
}

// ZIP 은 디자인 시스템, 페이지, 에셋을 담는다. hunsu 의 상태(.hunsu/)는 넣지 않는다.
func ZIP(w io.Writer, p *project.Project) error {
	listing, err := p.List()
	if err != nil {
		return err
	}
	archive := zip.NewWriter(w)
	for _, name := range append(listing.Files(), listing.Assets...) {
		if err := add(archive, p.FS(), name); err != nil {
			return err
		}
	}
	return archive.Close()
}

func add(archive *zip.Writer, fsys fs.FS, name string) error {
	info, err := fs.Stat(fsys, name)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name, header.Method = name, zip.Deflate
	f, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}
