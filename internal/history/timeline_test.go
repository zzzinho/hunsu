package history_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"hunsu/internal/history"
	"hunsu/internal/project"
	"hunsu/internal/project/projecttest"
)

type timelineFixture struct {
	t        *testing.T
	project  *project.Project
	timeline *history.Timeline
}

func newTimeline(t *testing.T, thumb history.Thumbnailer) *timelineFixture {
	p, _ := projecttest.New(t, map[string]string{"a.html": "1"})
	timeline, err := history.NewTimeline(p, thumb)
	must(t, err)
	return &timelineFixture{t: t, project: p, timeline: timeline}
}

func (f *timelineFixture) keep(label, page string) bool {
	saved, err := f.timeline.Keep(label, page)
	must(f.t, err)
	return saved
}

func (f *timelineFixture) list() string {
	versions, err := f.timeline.List()
	must(f.t, err)
	var out []string
	for _, v := range versions {
		out = append(out, fmt.Sprintf("%d:%s:%s:%v", v.ID, v.Label, v.Page, v.Thumb))
	}
	return strings.Join(out, " ")
}

func thumbOf(page string) ([]byte, error) { return []byte("thumb:" + page), nil }

func TestTimeline(t *testing.T) {
	tests := []struct {
		name  string
		thumb history.Thumbnailer
		run   func(f *timelineFixture) string
		want  string
	}{
		{"버전을 남기면 최신 버전부터 이름·대표 페이지와 함께 나온다", nil,
			func(f *timelineFixture) string {
				f.keep("처음", "a.html")
				setFiles(f.t, f.project, files{"a.html": "2"})
				f.keep("버튼 둥글게", "a.html")
				return f.list()
			},
			"2:버튼 둥글게:a.html:false 1:처음:a.html:false"},
		{"직전 버전과 내용이 같으면 남기지 않는다", nil,
			func(f *timelineFixture) string {
				f.keep("처음", "a.html")
				return fmt.Sprint(f.keep("다시", "a.html"), " ", f.list())
			},
			"false 1:처음:a.html:false"},
		{"디자인 파일이 하나도 없으면 남기지 않는다", nil,
			func(f *timelineFixture) string {
				setFiles(f.t, f.project, files{})
				return fmt.Sprint(f.keep("빈 폴더", ""), " ", f.list())
			},
			"false "},
		{"버전으로 복원하면 그때의 파일로 돌아가고 그 뒤에 생긴 페이지는 지운다", nil,
			func(f *timelineFixture) string {
				f.keep("처음", "a.html")
				setFiles(f.t, f.project, files{"a.html": "2", "b.html": "새 페이지"})
				f.keep("b 추가", "b.html")
				v, err := f.timeline.Restore(1)
				must(f.t, err)
				return fmt.Sprint(v.Label, " ", readFiles(f.t, f.project))
			},
			"처음 map[a.html:1]"},
		{"없는 버전은 복원하지 않는다", nil,
			func(f *timelineFixture) string {
				_, err := f.timeline.Restore(7)
				return fmt.Sprint(errors.Is(err, history.ErrNoVersion))
			},
			"true"},
		{"대표 페이지의 썸네일을 함께 남긴다", thumbOf,
			func(f *timelineFixture) string {
				f.keep("처음", "a.html")
				data, err := f.timeline.Thumbnail(1)
				must(f.t, err)
				return string(data) + " " + f.list()
			},
			"thumb:a.html 1:처음:a.html:true"},
		{"썸네일을 만들지 못해도 버전은 남긴다", func(string) ([]byte, error) { return nil, errors.New("Chrome 없음") },
			func(f *timelineFixture) string {
				f.keep("처음", "a.html")
				return f.list()
			},
			"1:처음:a.html:false"},
		{"100개를 넘으면 오래된 버전부터 지운다", nil,
			func(f *timelineFixture) string {
				for i := range 102 {
					setFiles(f.t, f.project, files{"a.html": fmt.Sprint(i)})
					f.keep(fmt.Sprint("v", i), "a.html")
				}
				versions, err := f.timeline.List()
				must(f.t, err)
				return fmt.Sprint(len(versions), " ", versions[0].ID, " ", versions[len(versions)-1].ID)
			},
			"100 102 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.run(newTimeline(t, tt.thumb)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
