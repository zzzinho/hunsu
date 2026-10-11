package chat

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"

	"hunsu/internal/history"
	"hunsu/internal/llm"
)

var (
	ErrVariantCount = errors.New("variants come in 2 or 3")
	ErrNoVariants   = errors.New("no variants to pick from")
)

// 안마다 방향을 달리 주어야 같은 요청에서도 서로 다른 안이 나온다.
var directions = []string{
	"Follow the request as faithfully as possible",
	"Solve the same request with bolder color, type and layout",
	"Solve the same request in a simpler, more restrained way",
}

type Variant struct {
	ID    int    `json:"id"`
	Page  string `json:"page"`
	Reply string `json:"reply"`
	Error string `json:"error,omitempty"`
}

type variantRun struct {
	turn     Turn
	variants []Variant
}

// SendVariants 는 디자인 파일 복사본마다 provider 를 동시에 실행해 count 개의 안을 만든다. 원본은 바꾸지 않는다.
// 복사본을 찍을 수 없어서 도구는 건네지 않는다. 안마다 새 대화로 시작한다.
func (s *Service) SendVariants(ctx context.Context, t Turn, count int, emit func(llm.Event)) error {
	if strings.TrimSpace(t.Prompt) == "" {
		return ErrEmptyPrompt
	}
	if count < 2 || count > len(directions) {
		return ErrVariantCount
	}
	if !s.mu.TryLock() {
		return ErrBusy
	}
	defer s.mu.Unlock()

	before, err := s.project.List()
	if err != nil {
		return err
	}
	dirs, err := s.variants.Prepare(count)
	if err != nil {
		return err
	}
	design := designUpdate(s.contents(before), nil, true)
	model := cmp.Or(t.Model, s.defaultModel())
	variants := make([]Variant, count)
	var emitMu sync.Mutex
	var wg sync.WaitGroup
	for i, dir := range dirs {
		wg.Go(func() {
			v := Variant{ID: i + 1}
			note := fmt.Sprintf("Take %d of %d. %s. Make it clearly different from the other takes.", v.ID, count, directions[i])
			req := llm.Request{
				Dir:          dir,
				Prompt:       userPrompt(promptInput{turn: t, design: design, assets: before.Assets, variant: note}),
				SystemPrompt: systemPrompt(),
				Model:        model,
			}
			err := s.llm.Run(ctx, req, func(e llm.Event) {
				if e.Kind == llm.Replied {
					v.Reply = e.Text
				}
				e.Variant = v.ID
				emitMu.Lock()
				defer emitMu.Unlock()
				emit(e)
			})
			if err != nil {
				v.Error = err.Error()
			}
			v.Page = s.variantPage(v.ID, before.Pages, t.Page)
			variants[i] = v
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return errors.Join(err, s.variants.Clear())
	}
	s.pending = &variantRun{turn: t, variants: variants}
	emit(llm.Event{Kind: Ended})
	return nil
}

func (s *Service) variantPage(id int, before []string, viewing string) string {
	pages, _ := s.variants.Pages(id)
	var created []string
	for _, page := range pages {
		if !slices.Contains(before, page) {
			created = append(created, page)
		}
	}
	return representative(created, viewing, pages)
}

// Variants 는 고르기를 기다리는 안이다. 없으면 비어 있다.
func (s *Service) Variants() []Variant {
	if s.pending == nil {
		return []Variant{}
	}
	return slices.Clone(s.pending.variants)
}

func (s *Service) VariantFiles(id int) (fs.FS, error) { return s.variants.FS(id) }

// ChooseVariant 는 안 id 를 지금 상태로 만든다. 고르기 전 상태는 되돌리기와 버전에, 나머지 안도 버전에 남는다.
func (s *Service) ChooseVariant(id int, page string) error {
	if !s.mu.TryLock() {
		return ErrBusy
	}
	defer s.mu.Unlock()
	run := s.pending
	if run == nil {
		return ErrNoVariants
	}
	if id < 1 || id > len(run.variants) || run.variants[id-1].Error != "" {
		return history.ErrNoVariant
	}
	if err := s.history.Save(); err != nil {
		return err
	}
	if _, err := s.timeline.Keep(history.Note{Kind: kindBeforeRequest}, page); err != nil {
		return err
	}
	text := truncate(run.turn.Prompt, labelLimit)
	// 버전은 지금 파일로 남기므로, 나머지 안을 차례로 적용하며 남긴 뒤 고른 안을 마지막에 적용한다.
	for _, v := range run.variants {
		if v.ID == id || v.Error != "" {
			continue
		}
		if err := s.keepVariant(v, history.Note{Kind: kindVariant, Text: text, Ref: v.ID}); err != nil {
			return err
		}
	}
	if err := s.keepVariant(run.variants[id-1], history.Note{Kind: kindChosen, Text: text, Ref: id}); err != nil {
		return err
	}
	s.pending = nil
	return s.variants.Clear()
}

func (s *Service) keepVariant(v Variant, note history.Note) error {
	if err := s.variants.Apply(v.ID); err != nil {
		return err
	}
	_, err := s.timeline.Keep(note, v.Page)
	return err
}

func (s *Service) DiscardVariants() error {
	if !s.mu.TryLock() {
		return ErrBusy
	}
	defer s.mu.Unlock()
	s.pending = nil
	return s.variants.Clear()
}
