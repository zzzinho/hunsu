package ui

import "sync"

const Open = "open"

const inboxSize = 4

type Selection struct {
	Selector string `json:"selector"`
	HTML     string `json:"html"`
}

type State struct {
	Page      string     `json:"page"`
	Selection *Selection `json:"selection"`
}

type Command struct {
	Kind string `json:"kind"`
	Page string `json:"page"`
}

type Hub struct {
	mu          sync.Mutex
	state       State
	subscribers map[chan Command]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: map[chan Command]struct{}{}}
}

func (h *Hub) Update(s State) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = s
}

func (h *Hub) Current() State {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (h *Hub) Subscribe() (<-chan Command, func()) {
	inbox := make(chan Command, inboxSize)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers[inbox] = struct{}{}
	return inbox, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subscribers, inbox)
	}
}

// 명령을 읽지 않는 브라우저 때문에 서버가 멈추지 않도록, 받을 자리가 없으면 그 브라우저는 건너뛴다.
func (h *Hub) Send(c Command) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	sent := 0
	for inbox := range h.subscribers {
		select {
		case inbox <- c:
			sent++
		default:
		}
	}
	return sent
}
