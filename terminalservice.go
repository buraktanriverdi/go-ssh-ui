package main

import (
	"fmt"

	"go-ssh-ui/internal/sshengine"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TerminalOutputEvent carries a chunk of raw terminal output for one
// session. Sessions can come from either sshengine backend (native or
// subprocess) - the frontend never needs to know which.
type TerminalOutputEvent struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"`
}

// TerminalClosedEvent fires once, when a session's underlying connection
// ends - either it never came up (dial/auth failure, Error set) or it did
// and then later disconnected (remote exit, network failure, a local
// Disconnect call).
type TerminalClosedEvent struct {
	SessionID string `json:"sessionId"`
	Error     string `json:"error,omitempty"`
}

// TerminalConnectedEvent fires once a session's shell is actually up and
// ready for input - Connect returns the session ID immediately, before
// dialing even starts, so the frontend can show a "connecting…" state until
// this arrives (or "terminal:closed" arrives instead, if it fails).
type TerminalConnectedEvent struct {
	SessionID string `json:"sessionId"`
}

func init() {
	application.RegisterEvent[TerminalOutputEvent]("terminal:output")
	application.RegisterEvent[TerminalConnectedEvent]("terminal:connected")
	application.RegisterEvent[TerminalClosedEvent]("terminal:closed")
	application.RegisterEvent[sshengine.Prompt]("terminal:prompt")
}

// TerminalService is the Wails-bound entry point for opening and driving
// terminal sessions. It just adapts sshengine's callback-based API to Wails
// events/bound methods - see internal/sshengine for the actual connection
// logic (native x/crypto/ssh vs. subprocess ssh binary).
type TerminalService struct {
	hosts *HostService
	mgr   *sshengine.Manager
}

// NewTerminalService adapts an already-constructed sshengine.Manager (see
// NewSSHManager - shared with FileService, so terminal tabs and file
// operations against the same structured host reuse one pooled
// *ssh.Client) to Wails events/bound methods.
func NewTerminalService(hosts *HostService, mgr *sshengine.Manager) *TerminalService {
	return &TerminalService{hosts: hosts, mgr: mgr}
}

// NewSSHManager wires sshengine to the shared HostService (host/jump
// lookups) and PasswordService (SENDPASS / stored-password auth) so it can
// resolve full connection chains on its own, and to the Wails events both
// TerminalService and FileService's callers listen on.
func NewSSHManager(hosts *HostService, passwords *PasswordService) *sshengine.Manager {
	return sshengine.NewManager(
		passwords,
		hosts,
		func(p sshengine.Prompt) {
			application.Get().Event.Emit("terminal:prompt", p)
		},
		func(sessionID, data string) {
			application.Get().Event.Emit("terminal:output", TerminalOutputEvent{SessionID: sessionID, Data: data})
		},
		func(sessionID string) {
			application.Get().Event.Emit("terminal:connected", TerminalConnectedEvent{SessionID: sessionID})
		},
		func(sessionID string, closeErr error) {
			ev := TerminalClosedEvent{SessionID: sessionID}
			if closeErr != nil {
				ev.Error = closeErr.Error()
			}
			application.Get().Event.Emit("terminal:closed", ev)
		},
	)
}

// ConnectRequest addresses a host the same way the tree UI does: by
// category path + name. That works even for hosts without a stable ID -
// i.e. every host that was never saved through go-ssh-ui's own forms,
// which today is every host in an existing ~/.go-ssh config.
type ConnectRequest struct {
	CategoryPath []string `json:"categoryPath"`
	Name         string   `json:"name"`
	Cols         int      `json:"cols"`
	Rows         int      `json:"rows"`
}

type ConnectResponse struct {
	SessionID string `json:"sessionId"`
	HostName  string `json:"hostName"`
}

// Connect resolves the requested host and returns a session ID right away;
// the actual dial/auth/shell-start happens in the background and is
// reported via "terminal:connected"/"terminal:closed"/"terminal:prompt"
// events on that ID (see TerminalConnectedEvent doc comment for why).
func (t *TerminalService) Connect(req ConnectRequest) (ConnectResponse, error) {
	host, ok := t.hosts.FindHostByPath(req.CategoryPath, req.Name)
	if !ok {
		return ConnectResponse{}, fmt.Errorf("host bulunamadı: %s", req.Name)
	}
	sessionID := t.mgr.Connect(host, req.Cols, req.Rows)
	return ConnectResponse{SessionID: sessionID, HostName: host.Name}, nil
}

// Write forwards keystrokes/pasted text from the frontend terminal to the
// remote shell.
func (t *TerminalService) Write(sessionID, data string) error {
	return t.mgr.Write(sessionID, data)
}

// Resize reports the frontend terminal's current size so the remote shell's
// PTY (and any full-screen program running in it) redraws correctly.
func (t *TerminalService) Resize(sessionID string, cols, rows int) error {
	return t.mgr.Resize(sessionID, cols, rows)
}

// Disconnect closes a session. A "terminal:closed" event still follows once
// teardown actually completes.
func (t *TerminalService) Disconnect(sessionID string) error {
	return t.mgr.Disconnect(sessionID)
}

// AnswerPrompt responds to a pending "terminal:prompt" event: keyboard-interactive
// answers, a private key passphrase, or a host-key trust decision.
func (t *TerminalService) AnswerPrompt(promptID string, values []string, trusted bool) bool {
	return t.mgr.AnswerPrompt(promptID, values, trusted)
}
