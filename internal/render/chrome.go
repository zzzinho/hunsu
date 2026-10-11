package render

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
)

var chromePaths = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
	"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
}

// Find 는 설치된 Chrome 계열 브라우저의 실행 파일을 찾는다. 없으면 빈 문자열이다.
func Find() string {
	for _, path := range chromePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// Chrome 은 headless Chrome 으로 디자인 페이지를 찍는다. Base 는 페이지 URL 의 앞부분이다.
type Chrome struct {
	Path string
	Base string
}

// Screenshot 은 page 를 width 너비로 렌더해 위에서부터 maxHeight 까지 PNG 로 찍는다.
func (c Chrome) Screenshot(ctx context.Context, page string, width, maxHeight int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "hunsu-chrome-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	toChrome, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	reader, fromChrome, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	// WebSocket 없이 표준 라이브러리만으로 DevTools 프로토콜을 쓰려고 파이프(fd 3, 4)로 연결한다.
	cmd := exec.Command(c.Path, "--headless=new", "--remote-debugging-pipe", "--user-data-dir="+dir,
		"--no-first-run", "--no-default-browser-check", "--hide-scrollbars", "--mute-audio", "about:blank")
	cmd.ExtraFiles = []*os.File{toChrome, fromChrome}
	err = cmd.Start()
	toChrome.Close()
	fromChrome.Close()
	if err != nil {
		writer.Close()
		reader.Close()
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { reader.Close() })
	defer func() {
		stop()
		writer.Close()
		reader.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()
	png, err := capture(&devtools{w: writer, r: bufio.NewReader(reader)}, c.Base+url.PathEscape(page), width, maxHeight)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return png, err
}

func capture(d *devtools, pageURL string, width, maxHeight int) ([]byte, error) {
	var target struct {
		TargetID string `json:"targetId"`
	}
	if err := d.call("Target.createTarget", map[string]any{"url": "about:blank"}, &target); err != nil {
		return nil, err
	}
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := d.call("Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true}, &attached); err != nil {
		return nil, err
	}
	d.session = attached.SessionID
	if err := d.call("Page.enable", nil, nil); err != nil {
		return nil, err
	}
	viewport := map[string]any{"width": width, "height": 800, "deviceScaleFactor": 1, "mobile": false}
	if err := d.call("Emulation.setDeviceMetricsOverride", viewport, nil); err != nil {
		return nil, err
	}
	var navigated struct {
		ErrorText string `json:"errorText"`
	}
	if err := d.call("Page.navigate", map[string]any{"url": pageURL}, &navigated); err != nil {
		return nil, err
	}
	if navigated.ErrorText != "" {
		return nil, fmt.Errorf("could not open %s: %s", pageURL, navigated.ErrorText)
	}
	if err := d.waitFor("Page.loadEventFired"); err != nil {
		return nil, err
	}
	fonts := map[string]any{"expression": "document.fonts.ready.then(() => true)", "awaitPromise": true}
	if err := d.call("Runtime.evaluate", fonts, nil); err != nil {
		return nil, err
	}
	var metrics struct {
		CSSContentSize struct {
			Height float64 `json:"height"`
		} `json:"cssContentSize"`
	}
	if err := d.call("Page.getLayoutMetrics", nil, &metrics); err != nil {
		return nil, err
	}
	var shot struct {
		Data []byte `json:"data"`
	}
	clip := map[string]any{"x": 0, "y": 0, "width": width, "height": math.Min(math.Ceil(metrics.CSSContentSize.Height), float64(maxHeight)), "scale": 1}
	err := d.call("Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": true, "clip": clip}, &shot)
	return shot.Data, err
}

type devtools struct {
	w       io.Writer
	r       *bufio.Reader
	session string
	lastID  int
}

type incoming struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (d *devtools) call(method string, params, result any) error {
	d.lastID++
	msg, err := json.Marshal(map[string]any{"id": d.lastID, "method": method, "params": params, "sessionId": d.session})
	if err != nil {
		return err
	}
	if _, err := d.w.Write(append(msg, 0)); err != nil {
		return err
	}
	for {
		in, err := d.read()
		if err != nil {
			return err
		}
		if in.ID != d.lastID {
			continue
		}
		if in.Error != nil {
			return fmt.Errorf("%s: %s", method, in.Error.Message)
		}
		if result == nil {
			return nil
		}
		return json.Unmarshal(in.Result, result)
	}
}

func (d *devtools) waitFor(event string) error {
	for {
		in, err := d.read()
		if err != nil || in.Method == event {
			return err
		}
	}
}

func (d *devtools) read() (incoming, error) {
	line, err := d.r.ReadBytes(0)
	if err != nil {
		return incoming{}, errors.Join(errors.New("lost the connection to Chrome"), err)
	}
	var in incoming
	err = json.Unmarshal(line[:len(line)-1], &in)
	return in, err
}
