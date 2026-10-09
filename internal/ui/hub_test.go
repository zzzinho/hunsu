package ui_test

import (
	"reflect"
	"testing"

	"hunsu/internal/ui"
)

var heading = &ui.Selection{Selector: "h1", HTML: "<h1>안녕</h1>"}

func TestHub_Current(t *testing.T) {
	tests := []struct {
		name    string
		updates []ui.State
		want    ui.State
	}{
		{"브라우저가 마지막으로 알린 화면 상태를 기억한다",
			[]ui.State{{Page: "a.html"}, {Page: "b.html", Selection: heading}},
			ui.State{Page: "b.html", Selection: heading}},
		{"아무것도 알리지 않았으면 빈 상태다", nil, ui.State{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := ui.NewHub()
			for _, s := range tt.updates {
				hub.Update(s)
			}
			if got := hub.Current(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Current() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHub_Send(t *testing.T) {
	tests := []struct {
		name         string
		connected    int
		disconnected int
		want         int
	}{
		{"명령을 연결된 모든 브라우저에 보낸다", 2, 0, 2},
		{"연결을 끊은 브라우저에는 보내지 않는다", 2, 1, 1},
		{"연결된 브라우저가 없으면 보낸 곳이 없다고 알린다", 0, 0, 0},
	}
	command := ui.Command{Kind: ui.Open, Page: "a.html"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := ui.NewHub()
			var inboxes []<-chan ui.Command
			for i := range tt.connected {
				inbox, cancel := hub.Subscribe()
				if i < tt.disconnected {
					cancel()
					continue
				}
				inboxes = append(inboxes, inbox)
			}
			if got := hub.Send(command); got != tt.want {
				t.Fatalf("Send() = %d, want %d", got, tt.want)
			}
			for _, inbox := range inboxes {
				select {
				case got := <-inbox:
					if got != command {
						t.Errorf("받은 명령 = %+v, want %+v", got, command)
					}
				default:
					t.Error("연결된 브라우저가 명령을 받지 못했다")
				}
			}
		})
	}
}
