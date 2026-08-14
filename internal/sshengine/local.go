package sshengine

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// localSession runs the user's own shell ($SHELL, falling back to
// /bin/bash) under a PTY - no SSH involved at all. It backs the toolbar's
// plain "Terminal" button, which opens a tab into the machine go-ssh-ui
// itself is running on, the same way a Terminal.app window would.
type localSession struct {
	id  string
	mgr *Manager

	cmd  *exec.Cmd
	ptmx *os.File

	mu     sync.Mutex
	closed bool
}

func (m *Manager) connectLocal(sessionID string, cols, rows int) (Session, error) {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	// "-l" starts a login shell, same as a fresh Terminal.app window, so
	// profile files (.zprofile/.bash_profile) run and PATH matches what the
	// user expects from a real terminal.
	cmd := exec.Command(shellPath(), "-l")
	cmd.Dir, _ = os.UserHomeDir()
	// go-ssh-ui is launched as a GUI app, so it doesn't inherit a TERM from
	// any shell - without one, the spawned shell falls back to plain
	// black-and-white output even though xterm.js on the other end fully
	// supports color (same reasoning as connectSubprocess).
	cmd.Env = envWithOverrides(map[string]string{
		"TERM":      "xterm-256color",
		"COLORTERM": "truecolor",
	})
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, fmt.Errorf("pty başlatılamadı: %w", err)
	}

	s := &localSession{id: sessionID, mgr: m, cmd: cmd, ptmx: ptmx}
	go s.pump()
	return s, nil
}

func (s *localSession) ID() string { return s.id }

func (s *localSession) Write(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("oturum kapalı")
	}
	_, err := s.ptmx.Write([]byte(data))
	return err
}

func (s *localSession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (s *localSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.ptmx.Close()
}

func (s *localSession) pump() {
	buf := make([]byte, 4096)
	for {
		n, readErr := s.ptmx.Read(buf)
		if n > 0 {
			s.mgr.emitOutput(s.id, string(buf[:n]))
		}
		if readErr != nil {
			_ = s.cmd.Wait()
			s.mgr.removeSession(s.id, nil)
			return
		}
	}
}
