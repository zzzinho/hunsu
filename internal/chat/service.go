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

	"hunsu/internal/claude"
	"hunsu/internal/history"
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

type Runner interface {
	Run(ctx context.Context, req claude.Request, emit func(claude.Event)) error
}

type Turn struct {
	Prompt    string        `json:"prompt"`
	Page      string        `json:"page"`
	Model     string        `json:"model"`
	Selection *ui.Selection `json:"selection"`
}

type Service struct {
	mu      sync.Mutex
	project *project.Project
	runner  Runner
	history *history.History
	session session
	seen    map[string][32]byte
}

func New(p *project.Project, r Runner) (*Service, error) {
	h, err := history.New(p)
	if err != nil {
		return nil, err
	}
	state, err := p.State()
	if err != nil {
		return nil, err
	}
	return &Service{project: p, runner: r, history: h, session: session{state}}, nil
}

func (s *Service) Send(ctx context.Context, t Turn, emit func(claude.Event)) error {
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
	req := claude.Request{
		Dir:          s.project.Dir(),
		Prompt:       userPrompt(t, s.editedSinceLastReply(current)),
		SystemPrompt: systemPrompt(current),
		SessionID:    s.session.load(),
		Model:        t.Model,
	}
	var sessionID string
	runErr := s.runner.Run(ctx, req, func(e claude.Event) {
		sessionID = cmp.Or(e.SessionID, sessionID)
		emit(e)
	})

	after, listErr := s.project.List()
	s.seen = fingerprints(s.contents(after))
	for _, page := range after.Pages {
		if !slices.Contains(before.Pages, page) {
			emit(claude.Event{Kind: Created, File: page})
		}
	}
	var saveErr error
	if sessionID != "" {
		saveErr = s.session.save(sessionID)
	}
	if err := cmp.Or(runErr, listErr, saveErr); err != nil {
		return err
	}
	emit(claude.Event{Kind: Ended})
	return nil
}

// 되돌린 내용은 claude 가 모르는 변경이므로 seen 을 갱신하지 않는다. 다음 요청에서 직접 고친 파일로 알려진다.
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
