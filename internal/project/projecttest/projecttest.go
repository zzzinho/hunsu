package projecttest

import (
	"os"
	"path/filepath"
	"testing"

	"hunsu/internal/project"
)

func New(t testing.TB, files map[string]string) (*project.Project, string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p, dir
}
