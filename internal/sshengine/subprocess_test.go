package sshengine

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go-ssh/config"
)

type fakePasswords struct{ m map[string]string }

func (f fakePasswords) Reveal(id string) (string, error) {
	if v, ok := f.m[id]; ok {
		return v, nil
	}
	return "", fmt.Errorf("not found: %s", id)
}

type fakeHosts struct{}

func (fakeHosts) FindHostByID(id string) (config.Host, bool) { return config.Host{}, false }

// capture collects a session's output and close status across goroutines,
// standing in for the Wails event emission TerminalService normally does.
type capture struct {
	mu       sync.Mutex
	output   strings.Builder
	closed   bool
	closeErr error
}

func (c *capture) onOutput(_, data string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.output.WriteString(data)
}

func (c *capture) onClosed(_ string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.closeErr = err
}

func (c *capture) snapshot() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.output.String(), c.closed
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for condition")
}

// TestSubprocessSessionEchoAndExit exercises the plain single-command path
// (no automation steps) end to end: connect, read output, observe the
// process exit being reported.
func TestSubprocessSessionEchoAndExit(t *testing.T) {
	cap := &capture{}
	m := NewManager(fakePasswords{}, fakeHosts{}, func(Prompt) {}, cap.onOutput, func(string) {}, cap.onClosed)

	host := config.Host{Name: "echo-test", Command: "echo ssh-hello-test"}
	sessionID := m.Connect(host, 80, 24)
	if sessionID == "" {
		t.Fatal("expected non-empty session id")
	}

	waitFor(t, 5*time.Second, func() bool {
		out, closed := cap.snapshot()
		return closed && strings.Contains(out, "ssh-hello-test")
	})
}

// TestSubprocessSessionSendExpectAutomation exercises the
// EXPECT/SENDPASS/WAIT automation loop against a long-lived `cat` process
// standing in for an interactive remote shell: cat echoes back whatever
// SENDPASS writes to its stdin, proving the automation and the stored
// password resolution both work.
func TestSubprocessSessionSendExpectAutomation(t *testing.T) {
	cap := &capture{}
	m := NewManager(
		fakePasswords{m: map[string]string{"secret": "hunter2"}},
		fakeHosts{},
		func(Prompt) {},
		cap.onOutput,
		func(string) {},
		cap.onClosed,
	)

	host := config.Host{
		Name: "cat-test",
		Commands: []string{
			"echo ssh-ready && cat",
			"EXPECT:ssh-ready",
			"SENDPASS:secret",
		},
	}
	sessionID := m.Connect(host, 80, 24)

	waitFor(t, 5*time.Second, func() bool {
		out, _ := cap.snapshot()
		return strings.Contains(out, "hunter2")
	})

	if err := m.Disconnect(sessionID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool {
		_, closed := cap.snapshot()
		return closed
	})
}
