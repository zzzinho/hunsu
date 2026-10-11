package render

import (
	"fmt"
)

const (
	defaultWidth = 1280
	minWidth     = 320
	maxWidth     = 2560
)

var ErrWidth = fmt.Errorf("width must be %d–%d px", minWidth, maxWidth)

// Width 는 렌더 너비를 정한다. 0 이면 데스크톱 기본 너비다.
func Width(width int) (int, error) {
	if width == 0 {
		return defaultWidth, nil
	}
	if width < minWidth || width > maxWidth {
		return 0, ErrWidth
	}
	return width, nil
}
