// Package sshengine drives terminal connections for go-ssh-ui. It has two
// engines behind one Session interface:
//
//   - native: golang.org/x/crypto/ssh, used for hosts with structured
//     connection fields (Host.HostAddr set). Dials jump-chains
//     programmatically and is the engine SFTP/broadcast (later phases) build
//     on.
//   - subprocess: shells out to the system `ssh` binary under a PTY, used
//     for every host that only has the legacy Command/Commands string(s)
//     go-ssh's CLI already understands - which today is every host in an
//     existing ~/.go-ssh config, since none of them were created through
//     go-ssh-ui's structured host form yet. This inherits the real ssh
//     binary's own known_hosts handling, ssh-agent, ProxyCommand, and the
//     go-ssh/ssh SEND/SENDPASS/WAIT/EXPECT/INTERACT automation verbatim, so
//     every host that already works in the CLI works here immediately.
//
// Both engines stream output through the same OutputFunc/ClosedFunc
// callbacks, keyed by session ID, so the frontend terminal component and
// TerminalService don't need to know which engine is behind a given session.
package sshengine

import (
	"fmt"
	"sync"

	"go-ssh/config"

	"github.com/google/uuid"
)

// Session is one live terminal connection.
type Session interface {
	ID() string
	Write(data string) error
	Resize(cols, rows int) error
	Close() error
}

// PasswordResolver looks up a stored password by ID. *PasswordService
// already implements this.
type PasswordResolver interface {
	Reveal(id string) (string, error)
}

// HostFinder resolves a host by its stable ID, used to look up jump hosts
// referenced by Host.JumpVia. *HostService implements this.
type HostFinder interface {
	FindHostByID(id string) (config.Host, bool)
}

// recorderSink is the shape of a "Kayıt" (record) session watcher -
// *record.Recorder implements this structurally. Kept as a local interface
// so sshengine doesn't need to import internal/record just to feed it.
type recorderSink interface {
	FeedInput(data string)
	FeedOutput(data string)
}

// Manager owns every live session and the shared native-engine connection
// pool (so multiple sessions to the same host reuse one authenticated
// ssh.Client instead of re-dialing/re-authenticating per tab).
type Manager struct {
	mu        sync.Mutex
	sessions  map[string]Session
	clients   map[string]*pooledClient
	recorders map[string]recorderSink

	passwords PasswordResolver
	hosts     HostFinder
	broker    *Broker

	onOutput    func(sessionID, data string)
	onConnected func(sessionID string)
	onClosed    func(sessionID string, err error)
}

func NewManager(passwords PasswordResolver, hosts HostFinder, emitPrompt func(Prompt), onOutput func(sessionID, data string), onConnected func(sessionID string), onClosed func(sessionID string, err error)) *Manager {
	return &Manager{
		sessions:    make(map[string]Session),
		clients:     make(map[string]*pooledClient),
		recorders:   make(map[string]recorderSink),
		passwords:   passwords,
		hosts:       hosts,
		broker:      NewBroker(emitPrompt),
		onOutput:    onOutput,
		onConnected: onConnected,
		onClosed:    onClosed,
	}
}

// SetRecorder attaches a "Kayıt" watcher to a live session - from then on,
// every Write (frontend keystrokes) and emitted output for that session ID
// is also fed to it. Only one recorder per session; a second call replaces
// the first.
func (m *Manager) SetRecorder(sessionID string, r recorderSink) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recorders[sessionID] = r
}

// ClearRecorder detaches a session's recorder, if any.
func (m *Manager) ClearRecorder(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.recorders, sessionID)
}

func (m *Manager) recorderFor(sessionID string) recorderSink {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recorders[sessionID]
}

// Connect returns a session ID immediately and dials/authenticates in the
// background, since that can take several seconds and may need one or more
// round trips to the frontend (host-key trust, 2FA) before it either
// succeeds ("terminal:connected") or fails ("terminal:closed" with an
// error) - both reported as events on the returned session ID, which the
// frontend starts listening on right away rather than waiting for this call
// to return the way it'd wait for a quick RPC.
func (m *Manager) Connect(host config.Host, cols, rows int) string {
	sessionID := uuid.NewString()
	go m.finishConnect(sessionID, func() (Session, error) {
		if host.HostAddr != "" {
			return m.connectNative(sessionID, host, cols, rows)
		}
		return m.connectSubprocess(sessionID, host, cols, rows)
	})
	return sessionID
}

// ConnectLocal is Connect's counterpart for a plain local shell tab - no
// host, no SSH, just $SHELL under a PTY on the machine go-ssh-ui itself runs
// on. It follows the same immediate-session-ID / background-start /
// event-driven handshake as Connect so the frontend's TerminalPane can treat
// both identically.
func (m *Manager) ConnectLocal(cols, rows int) string {
	sessionID := uuid.NewString()
	go m.finishConnect(sessionID, func() (Session, error) {
		return m.connectLocal(sessionID, cols, rows)
	})
	return sessionID
}

// finishConnect runs dial in the background and reports its outcome through
// the manager's callbacks - shared by Connect and ConnectLocal, which only
// differ in how they build the Session.
func (m *Manager) finishConnect(sessionID string, dial func() (Session, error)) {
	sess, err := dial()
	if err != nil {
		m.broker.CancelSession(sessionID)
		if m.onClosed != nil {
			m.onClosed(sessionID, err)
		}
		return
	}

	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	if m.onConnected != nil {
		m.onConnected(sessionID)
	}
}

func (m *Manager) session(id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("oturum bulunamadı: %s", id)
	}
	return s, nil
}

func (m *Manager) Write(id, data string) error {
	if r := m.recorderFor(id); r != nil {
		r.FeedInput(data)
	}
	s, err := m.session(id)
	if err != nil {
		return err
	}
	return s.Write(data)
}

func (m *Manager) Resize(id string, cols, rows int) error {
	s, err := m.session(id)
	if err != nil {
		return err
	}
	return s.Resize(cols, rows)
}

func (m *Manager) Disconnect(id string) error {
	s, err := m.session(id)
	if err != nil {
		return err
	}
	m.broker.CancelSession(id)
	return s.Close()
}

// AnswerPrompt delivers a frontend response to a pending Ask() call.
func (m *Manager) AnswerPrompt(promptID string, values []string, trusted bool) bool {
	return m.broker.Answer(promptID, Answer{Values: values, Trusted: trusted})
}

// removeSession drops the bookkeeping entry and fires onClosed. Engines call
// this once their underlying connection actually ends (process exit /
// channel close), not when Close() is merely requested.
func (m *Manager) removeSession(id string, closeErr error) {
	m.mu.Lock()
	delete(m.sessions, id)
	delete(m.recorders, id)
	m.mu.Unlock()
	m.broker.CancelSession(id)
	if m.onClosed != nil {
		m.onClosed(id, closeErr)
	}
}

func (m *Manager) emitOutput(id, data string) {
	if r := m.recorderFor(id); r != nil {
		r.FeedOutput(data)
	}
	if m.onOutput != nil {
		m.onOutput(id, data)
	}
}
