package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"hunsu/internal/mcp"
)

func TestLoadToken(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		want     string
	}{
		{"토큰 파일이 없으면 새로 만들어 본인만 읽을 수 있게 저장한다", "", ""},
		{"토큰 파일이 있으면 그 토큰을 그대로 쓴다", "saved-token", "saved-token"},
		{"토큰 파일이 비어 있으면 새로 만든다", "\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hunsu", "mcp-token")
			if tt.existing != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tt.existing), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			token, err := mcp.LoadToken(path)
			saved, _ := os.ReadFile(path)
			info, statErr := os.Stat(path)
			fresh := tt.want == "" && len(token) == 64
			if err != nil || statErr != nil || string(saved) != token || info.Mode().Perm() != 0o600 || !(fresh || token == tt.want) {
				t.Errorf("LoadToken() = %q, %v; 저장 = %q, 권한 = %v", token, err, saved, info.Mode().Perm())
			}
		})
	}
}
