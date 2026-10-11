package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MCP 클라이언트에 한 번 등록한 주소와 헤더가 계속 쓰이도록 토큰을 파일에 남겨 재사용한다.
// 파일이 비어 있으면(쓰다가 멈춘 경우) 새로 만든다. 빈 토큰은 어떤 요청과도 맞지 않는다.
func LoadToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if token := strings.TrimSpace(string(data)); err == nil && token != "" {
		return token, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	secret := make([]byte, 32)
	rand.Read(secret)
	token := hex.EncodeToString(secret)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return token, os.WriteFile(path, []byte(token), 0o600)
}
