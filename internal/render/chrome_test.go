package render_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"hunsu/internal/render"
)

// 테스트 바이너리를 가짜 Chrome 으로 다시 실행한다. 받은 URL·너비·높이를 PNG 대신 돌려준다.
func TestMain(m *testing.M) {
	if mode := os.Getenv("HUNSU_FAKE_CHROME"); mode != "" {
		fakeChrome(mode, os.Getenv("HUNSU_FAKE_HEIGHT"))
		return
	}
	os.Exit(m.Run())
}

func fakeChrome(mode, height string) {
	in := bufio.NewReader(os.NewFile(3, "in"))
	out := os.NewFile(4, "out")
	send := func(v any) {
		b, _ := json.Marshal(v)
		out.Write(append(b, 0))
	}
	var url string
	for {
		line, err := in.ReadBytes(0)
		if err != nil {
			return
		}
		var msg struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params struct {
				URL  string `json:"url"`
				Clip struct {
					Width, Height float64
				} `json:"clip"`
			} `json:"params"`
		}
		json.Unmarshal(line[:len(line)-1], &msg)
		result := map[string]any{}
		switch msg.Method {
		case "Target.createTarget":
			result["targetId"] = "t"
		case "Target.attachToTarget":
			result["sessionId"] = "s"
		case "Page.navigate":
			url = msg.Params.URL
		case "Page.getLayoutMetrics":
			h, _ := strconv.Atoi(height)
			result["cssContentSize"] = map[string]any{"height": h}
		case "Page.captureScreenshot":
			result["data"] = []byte(fmt.Sprintf("%s %vx%v", url, msg.Params.Clip.Width, msg.Params.Clip.Height))
		}
		send(map[string]any{"id": msg.ID, "result": result})
		if msg.Method == "Page.navigate" && mode != "hang" {
			send(map[string]any{"method": "Page.loadEventFired", "sessionId": "s", "params": map[string]any{}})
		}
	}
}

func TestChrome_Screenshot(t *testing.T) {
	tests := []struct {
		name    string
		chrome  string
		mode    string
		height  string
		page    string
		width   int
		want    string
		wantErr error
	}{
		{"페이지를 요청한 너비로 렌더해 PNG 로 돌려준다", os.Args[0], "ok", "1000", "a.html", 390, "http://hunsu.test/p/a.html 390x1000", nil},
		{"파일 이름의 공백은 URL 에 맞게 바꾼다", os.Args[0], "ok", "1000", "my page.html", 1280, "http://hunsu.test/p/my%20page.html 1280x1000", nil},
		{"페이지가 4000px 보다 길면 위에서 4000px 까지 찍는다", os.Args[0], "ok", "9000", "a.html", 1280, "http://hunsu.test/p/a.html 1280x4000", nil},
		{"페이지가 다 불러와지지 않으면 제한 시간에 멈춘다", os.Args[0], "hang", "1000", "a.html", 1280, "", context.DeadlineExceeded},
		{"Chrome 을 실행할 수 없으면 실패를 알린다", "/없는/chrome", "ok", "1000", "a.html", 1280, "", os.ErrNotExist},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HUNSU_FAKE_CHROME", tt.mode)
			t.Setenv("HUNSU_FAKE_HEIGHT", tt.height)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			got, err := render.Chrome{Path: tt.chrome, Base: "http://hunsu.test/p/"}.Screenshot(ctx, tt.page, tt.width)
			if string(got) != tt.want || !errors.Is(err, tt.wantErr) {
				t.Errorf("Screenshot(%q, %d) = %q, %v; want %q, %v", tt.page, tt.width, got, err, tt.want, tt.wantErr)
			}
		})
	}
}
