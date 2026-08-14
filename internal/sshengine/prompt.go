package sshengine

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// PromptKind distinguishes the synchronous "ask the frontend and wait" flows
// the native engine needs mid-handshake.
type PromptKind string

const (
	PromptKeyboardInteractive PromptKind = "keyboard-interactive"
	PromptHostKey             PromptKind = "host-key"
	PromptKeyPassphrase       PromptKind = "key-passphrase"
)

// Prompt is emitted to the frontend (over a Wails event) and answered via
// Broker.Answer, called from a Wails-bound method.
type Prompt struct {
	ID          string     `json:"id"`
	SessionID   string     `json:"sessionId"`
	Kind        PromptKind `json:"kind"`
	Name        string     `json:"name"`
	Instruction string     `json:"instruction"`
	Questions   []string   `json:"questions"`
	Echo        []bool     `json:"echo"`

	// Host-key specific fields (Kind == PromptHostKey).
	Hostname    string `json:"hostname,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	KeyType     string `json:"keyType,omitempty"`
	IsChanged   bool   `json:"isChanged,omitempty"`
}

// Answer is what the frontend sends back for a Prompt.
type Answer struct {
	Values  []string
	Trusted bool
}

// ErrPromptCancelled is returned to the blocked SSH callback when a prompt
// is answered negatively or its session is torn down before answering.
var ErrPromptCancelled = errors.New("kullanıcı isteği iptal etti")

type pending struct {
	sessionID string
	ch        chan promptResult
}

type promptResult struct {
	answer Answer
	err    error
}

// Broker bridges a blocking Go call (made from deep inside the SSH
// handshake, on its own goroutine) to an async frontend dialog: Ask emits an
// event and blocks until Answer(id, ...) is called back from the bound
// Wails method, or the owning session is cancelled.
type Broker struct {
	mu      sync.Mutex
	pending map[string]pending
	emit    func(Prompt)
}

func NewBroker(emit func(Prompt)) *Broker {
	return &Broker{pending: make(map[string]pending), emit: emit}
}

// Ask blocks the calling goroutine until the frontend answers (or the
// session is cancelled). Safe to call concurrently for different prompts.
func (b *Broker) Ask(p Prompt) (Answer, error) {
	p.ID = uuid.NewString()
	ch := make(chan promptResult, 1)

	b.mu.Lock()
	b.pending[p.ID] = pending{sessionID: p.SessionID, ch: ch}
	b.mu.Unlock()

	b.emit(p)

	result := <-ch
	return result.answer, result.err
}

// Answer delivers a frontend response to the matching pending Ask call.
// Returns false if no prompt with that ID is currently pending (e.g. it was
// already cancelled).
func (b *Broker) Answer(id string, a Answer) bool {
	b.mu.Lock()
	p, ok := b.pending[id]
	if ok {
		delete(b.pending, id)
	}
	b.mu.Unlock()
	if !ok {
		return false
	}
	p.ch <- promptResult{answer: a}
	return true
}

// CancelSession aborts every prompt still pending for a session, unblocking
// whatever SSH callback is waiting on it. Call this when a session is
// disconnected or fails so its goroutines don't leak forever.
func (b *Broker) CancelSession(sessionID string) {
	b.mu.Lock()
	var toCancel []chan promptResult
	for id, p := range b.pending {
		if p.sessionID == sessionID {
			toCancel = append(toCancel, p.ch)
			delete(b.pending, id)
		}
	}
	b.mu.Unlock()
	for _, ch := range toCancel {
		ch <- promptResult{err: ErrPromptCancelled}
	}
}
