package sshengine

import (
	"fmt"
	"io"
	"sync"
	"time"

	"go-ssh/config"

	"golang.org/x/crypto/ssh"
)

// nativeSession is a shell channel opened over a (possibly jump-chained)
// *ssh.Client dialed by this engine itself, as opposed to subprocessSession
// which shells out to the system ssh binary.
type nativeSession struct {
	id  string
	mgr *Manager

	client    *ssh.Client
	clientKey string
	session   *ssh.Session
	stdin     io.WriteCloser

	mu          sync.Mutex
	writeClosed bool

	done        chan struct{}
	cleanupOnce sync.Once
}

func (m *Manager) connectNative(sessionID string, host config.Host, cols, rows int) (Session, error) {
	client, clientKey, err := m.dialChain(sessionID, host)
	if err != nil {
		return nil, err
	}

	sess, err := client.NewSession()
	if err != nil {
		m.releaseClient(clientKey)
		return nil, fmt.Errorf("oturum açılamadı: %w", err)
	}

	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = sess.Close()
		m.releaseClient(clientKey)
		return nil, fmt.Errorf("pty istenemedi: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = sess.Close()
		m.releaseClient(clientKey)
		return nil, err
	}
	// A PTY-backed remote shell writes both stdout and stderr through the
	// same pty device server-side, so a separate StderrPipe carries nothing
	// useful here - one combined stream is exactly what a real terminal
	// (and xterm.js) expects anyway.
	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = sess.Close()
		m.releaseClient(clientKey)
		return nil, err
	}

	if err := sess.Shell(); err != nil {
		_ = sess.Close()
		m.releaseClient(clientKey)
		return nil, fmt.Errorf("shell başlatılamadı: %w", err)
	}

	ns := &nativeSession{
		id:        sessionID,
		mgr:       m,
		client:    client,
		clientKey: clientKey,
		session:   sess,
		stdin:     stdin,
		done:      make(chan struct{}),
	}

	go ns.pumpOutput(stdout)
	go ns.keepalive()
	go ns.waitForExit()

	return ns, nil
}

func (s *nativeSession) pumpOutput(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			s.mgr.emitOutput(s.id, string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

// keepalive sends periodic no-op requests so idle sessions survive NATs and
// server-side idle timeouts - x/crypto/ssh has no equivalent of OpenSSH's
// ServerAliveInterval built in.
func (s *nativeSession) keepalive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			if _, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		}
	}
}

// waitForExit blocks until the remote shell exits (naturally, or because
// Close() tore down the channel) and is the single place that releases the
// pooled client and unregisters the session - cleanupOnce means it doesn't
// matter whether the remote end or the local Close() call triggers it first.
func (s *nativeSession) waitForExit() {
	err := s.session.Wait()
	s.cleanup(err)
}

func (s *nativeSession) cleanup(err error) {
	s.cleanupOnce.Do(func() {
		close(s.done)
		s.mgr.releaseClient(s.clientKey)
		s.mgr.removeSession(s.id, err)
	})
}

func (s *nativeSession) ID() string { return s.id }

func (s *nativeSession) Write(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeClosed {
		return fmt.Errorf("oturum kapalı")
	}
	_, err := s.stdin.Write([]byte(data))
	return err
}

func (s *nativeSession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return s.session.WindowChange(rows, cols)
}

func (s *nativeSession) Close() error {
	s.mu.Lock()
	s.writeClosed = true
	s.mu.Unlock()
	// This unblocks session.Wait() in waitForExit, which performs the
	// actual cleanup exactly once (see cleanupOnce).
	return s.session.Close()
}
