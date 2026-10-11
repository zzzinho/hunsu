package chat

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"

	"hunsu/internal/history"
	"hunsu/internal/llm"
	"hunsu/internal/project"
	"hunsu/internal/ui"
)

const (
	Created = "created"
	Ended   = "end"
)

// 버전의 종류. 화면이 종류에 맞는 문구로 바꾼다.
const (
	kindRequest       = "request"
	kindBeforeRequest = "before-request"
	kindSaved         = "saved"
	kindBeforeRestore = "before-restore"
	kindRestore       = "restore"
	kindVariant       = "variant"
	kindChosen        = "chosen"
	labelLimit        = 60
)

var (
	ErrBusy        = errors.New("already handling a request")
	ErrEmptyPrompt = errors.New("the request is empty")
)

type Turn struct {
	Prompt    string        `json:"prompt"`
	Page      string        `json:"page"`
	Model     string        `json:"model"`
	Selection *ui.Selection `json:"selection"`
}

type Service struct {
	mu       sync.Mutex
	project  *project.Project
	llm      llm.Provider
	history  *history.History
	timeline *history.Timeline
	variants *history.Variants
	pending  *variantRun
	session  session
	seen     map[string][32]byte
	tools    *llm.MCPServer
}

type Options struct {
	// Thumbnail 이 nil 이면 버전에 썸네일을 남기지 않는다.
	Thumbnail history.Thumbnailer
	// Tools 는 요청마다 LLM 에게 건넬 hunsu 도구 서버다. nil 이면 건네지 않는다.
	Tools *llm.MCPServer
}

func New(p *project.Project, provider llm.Provider, opts Options) (*Service, error) {
	h, err := history.New(p)
	if err != nil {
		return nil, err
	}
	timeline, err := history.NewTimeline(p, opts.Thumbnail)
	if err != nil {
		return nil, err
	}
	variants, err := history.NewVariants(p)
	if err != nil {
		return nil, err
	}
	state, err := p.State()
	if err != nil {
		return nil, err
	}
	return &Service{project: p, llm: provider, history: h, timeline: timeline, variants: variants, session: newSession(state, provider.Name()), tools: opts.Tools}, nil
}

func (s *Service) Models() []llm.Model { return s.llm.Models() }

type ProviderStatus struct {
	Provider string `json:"provider"`
	llm.Status
}

func (s *Service) Status(ctx context.Context) ProviderStatus {
	return ProviderStatus{Provider: s.llm.Name(), Status: s.llm.Status(ctx)}
}

func (s *Service) Send(ctx context.Context, t Turn, emit func(llm.Event)) error {
	if strings.TrimSpace(t.Prompt) == "" {
		return ErrEmptyPrompt
	}
	if !s.mu.TryLock() {
		return ErrBusy
	}
	defer s.mu.Unlock()

	before, err := s.project.List()
	if err != nil {
		return err
	}
	current := s.contents(before)
	if err := s.history.Save(); err != nil {
		return err
	}
	if _, err := s.timeline.Keep(history.Note{Kind: kindBeforeRequest}, representative(nil, t.Page, before.Pages)); err != nil {
		return err
	}
	edited := s.editedSinceLastReply(current)
	req := llm.Request{
		Dir:          s.project.Dir(),
		Prompt:       userPrompt(promptInput{turn: t, edited: edited, design: designUpdate(current, edited, s.seen == nil), assets: before.Assets}),
		SystemPrompt: systemPrompt(),
		SessionID:    s.session.load(),
		Model:        cmp.Or(t.Model, s.defaultModel()),
		Tools:        s.tools,
	}
	var sessionID string
	runErr := s.llm.Run(ctx, req, func(e llm.Event) {
		sessionID = cmp.Or(e.SessionID, sessionID)
		emit(e)
	})

	after, listErr := s.project.List()
	s.seen = fingerprints(s.contents(after))
	var created []string
	for _, page := range after.Pages {
		if !slices.Contains(before.Pages, page) {
			created = append(created, page)
			emit(llm.Event{Kind: Created, File: page})
		}
	}
	var saveErr error
	if sessionID != "" {
		saveErr = s.session.save(sessionID)
	}
	if err := cmp.Or(runErr, listErr, saveErr); err != nil {
		return err
	}
	if _, err := s.timeline.Keep(history.Note{Kind: kindRequest, Text: truncate(t.Prompt, labelLimit)}, representative(created, t.Page, after.Pages)); err != nil {
		return err
	}
	emit(llm.Event{Kind: Ended})
	return nil
}

// 버전의 썸네일로 찍을 페이지: 새로 만든 페이지 → 보고 있던 페이지 → 첫 페이지.
func representative(created []string, viewing string, pages []string) string {
	switch {
	case len(created) > 0:
		return created[0]
	case slices.Contains(pages, viewing):
		return viewing
	case len(pages) > 0:
		return pages[0]
	}
	return ""
}

func (s *Service) Versions() ([]history.Version, error) { return s.timeline.List() }

func (s *Service) Thumbnail(id int) ([]byte, error) { return s.timeline.Thumbnail(id) }

// SaveVersion 은 지금 상태를 버전으로 남긴다. 직전 버전과 같으면 남기지 않고 false 다.
func (s *Service) SaveVersion(page string) (bool, error) {
	if !s.mu.TryLock() {
		return false, ErrBusy
	}
	defer s.mu.Unlock()
	return s.timeline.Keep(history.Note{Kind: kindSaved}, page)
}

// Restore 는 버전 id 의 파일로 돌아간다. 복원 전 상태는 되돌리기와 버전에 남고, 복원 결과도 새 버전이 된다.
func (s *Service) Restore(id int, page string) error {
	if !s.mu.TryLock() {
		return ErrBusy
	}
	defer s.mu.Unlock()
	v, err := s.timeline.Version(id)
	if err != nil {
		return err
	}
	if err := s.history.Save(); err != nil {
		return err
	}
	if _, err := s.timeline.Keep(history.Note{Kind: kindBeforeRestore}, page); err != nil {
		return err
	}
	if _, err := s.timeline.Restore(id); err != nil {
		return err
	}
	_, err = s.timeline.Keep(history.Note{Kind: kindRestore, Ref: id}, v.Page)
	return err
}

func (s *Service) defaultModel() string {
	if models := s.llm.Models(); len(models) > 0 {
		return models[0].ID
	}
	return ""
}

// 되돌린 내용은 LLM 이 모르는 변경이므로 seen 을 갱신하지 않는다. 다음 요청에서 직접 고친 파일로 알려진다.
func (s *Service) Undo() error {
	if !s.mu.TryLock() {
		return ErrBusy
	}
	defer s.mu.Unlock()
	return s.history.Undo()
}

func (s *Service) Checkpoint() error {
	if !s.mu.TryLock() {
		return ErrBusy
	}
	defer s.mu.Unlock()
	return s.history.Save()
}

func (s *Service) Reset() error {
	if !s.mu.TryLock() {
		return ErrBusy
	}
	defer s.mu.Unlock()
	s.seen = nil
	return s.session.reset()
}

func (s *Service) contents(listing project.Listing) map[string][]byte {
	files := map[string][]byte{}
	for _, name := range listing.Files() {
		if data, err := s.project.Read(name); err == nil {
			files[name] = data
		}
	}
	return files
}

func (s *Service) editedSinceLastReply(current map[string][]byte) []string {
	if s.seen == nil {
		return nil
	}
	var edited []string
	now := fingerprints(current)
	for _, name := range slices.Sorted(maps.Keys(now)) {
		if s.seen[name] != now[name] {
			edited = append(edited, name)
		}
	}
	return edited
}

func fingerprints(files map[string][]byte) map[string][32]byte {
	prints := make(map[string][32]byte, len(files))
	for name, data := range files {
		prints[name] = sha256.Sum256(data)
	}
	return prints
}
