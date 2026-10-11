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

var (
	ErrBusy        = errors.New("이미 요청을 처리하는 중")
	ErrEmptyPrompt = errors.New("요청이 비어 있다")
)

type Turn struct {
	Prompt    string        `json:"prompt"`
	Page      string        `json:"page"`
	Model     string        `json:"model"`
	Selection *ui.Selection `json:"selection"`
}

type Service struct {
	mu      sync.Mutex
	project *project.Project
	llm     llm.Provider
	history *history.History
	session session
	seen    map[string][32]byte
}

func New(p *project.Project, provider llm.Provider) (*Service, error) {
	h, err := history.New(p)
	if err != nil {
		return nil, err
	}
	state, err := p.State()
	if err != nil {
		return nil, err
	}
	return &Service{project: p, llm: provider, history: h, session: session{state}}, nil
}

func (s *Service) Models() []llm.Model { return s.llm.Models() }

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
	edited := s.editedSinceLastReply(current)
	req := llm.Request{
		Dir:          s.project.Dir(),
		Prompt:       userPrompt(t, edited, designUpdate(current, edited, s.seen == nil), before.Assets),
		SystemPrompt: systemPrompt(),
		SessionID:    s.session.load(),
		Model:        cmp.Or(t.Model, s.defaultModel()),
	}
	var sessionID string
	runErr := s.llm.Run(ctx, req, func(e llm.Event) {
		sessionID = cmp.Or(e.SessionID, sessionID)
		emit(e)
	})

	after, listErr := s.project.List()
	s.seen = fingerprints(s.contents(after))
	for _, page := range after.Pages {
		if !slices.Contains(before.Pages, page) {
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
	emit(llm.Event{Kind: Ended})
	return nil
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
