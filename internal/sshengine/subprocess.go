package sshengine

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go-ssh/config"
	gossh "go-ssh/ssh"

	"github.com/creack/pty"
)

// subprocessSession shells out to the system `ssh` binary under a PTY,
// exactly like go-ssh's CLI does, and pipes its output to the frontend
// instead of the process's own stdout. It reuses go-ssh/ssh's exported
// parsing helpers (EnsureSSHAutoAcceptHostKeys, ParseCommands) so the
// SEND/SENDPASS/WAIT/EXPECT/INTERACT semantics stay identical to the CLI's.
type subprocessSession struct {
	id  string
	mgr *Manager

	cmd  *exec.Cmd
	ptmx *os.File

	mu     sync.Mutex
	closed bool
}

func (m *Manager) connectSubprocess(sessionID string, host config.Host, cols, rows int) (Session, error) {
	commands := host.GetCommands()
	if len(commands) == 0 {
		return nil, fmt.Errorf("host için komut tanımlı değil")
	}
	if len(commands) == 1 {
		if err := gossh.ValidateCommand(commands[0]); err != nil {
			return nil, err
		}
	} else if err := gossh.ValidateCommands(commands); err != nil {
		return nil, err
	}

	parsed := gossh.ParseCommands(commands)

	var execCmd string
	startIdx := -1
	for i, pc := range parsed {
		if pc.Type == gossh.CommandTypeExec {
			execCmd = pc.Value
			startIdx = i + 1
			break
		}
	}
	if execCmd == "" {
		return nil, fmt.Errorf("çalıştırılabilir bir ssh komutu bulunamadı")
	}
	execCmd = gossh.EnsureSSHAutoAcceptHostKeys(execCmd)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	cmd := exec.Command(shell, "-c", execCmd)
	// go-ssh-ui is launched as a GUI app, not from a terminal, so it doesn't
	// inherit a TERM from any shell - without one, the spawned shell (and
	// anything it SSHes into) can't tell it's talking to a color-capable
	// terminal and falls back to plain black-and-white output even though
	// xterm.js on the other end fully supports color.
	cmd.Env = envWithOverrides(map[string]string{
		"TERM":      "xterm-256color",
		"COLORTERM": "truecolor",
	})
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, fmt.Errorf("pty başlatılamadı: %w", err)
	}

	s := &subprocessSession{id: sessionID, mgr: m, cmd: cmd, ptmx: ptmx}
	go s.pumpAndAutomate(parsed[startIdx:])

	return s, nil
}

func (s *subprocessSession) ID() string { return s.id }

func (s *subprocessSession) Write(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("oturum kapalı")
	}
	_, err := s.ptmx.Write([]byte(data))
	return err
}

func (s *subprocessSession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (s *subprocessSession) Close() error {
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

// pumpAndAutomate runs the SEND/SENDPASS/WAIT/EXPECT/INTERACT automation
// script (if any) on its own goroutine while reading ptmx output on the
// calling goroutine - there must be exactly one reader of ptmx, since both
// "stream to the frontend" and "watch for EXPECT text" need to see every
// byte, so the automation side watches a shared buffer instead of reading
// ptmx itself (mirrors go-ssh/ssh.go's ConnectInteractive).
func (s *subprocessSession) pumpAndAutomate(steps []gossh.ParsedCommand) {
	var (
		outputMu      sync.Mutex
		outputBuffer  strings.Builder
		bufferMarkPos int
	)
	outputChan := make(chan struct{}, 1)
	notifyOutput := func() {
		select {
		case outputChan <- struct{}{}:
		default:
		}
	}

	go s.runAutomation(steps, &outputMu, &outputBuffer, &bufferMarkPos, outputChan)

	buf := make([]byte, 4096)
	for {
		n, readErr := s.ptmx.Read(buf)
		if n > 0 {
			data := string(buf[:n])
			s.mgr.emitOutput(s.id, data)

			outputMu.Lock()
			outputBuffer.WriteString(data)
			// Keep the buffer bounded; clamp the mark if the trim moved it
			// out of range instead of letting a later slice panic.
			const maxBuffer = 10240
			if outputBuffer.Len() > maxBuffer {
				trimmed := outputBuffer.String()
				trimmed = trimmed[len(trimmed)-maxBuffer:]
				dropped := outputBuffer.Len() - len(trimmed)
				outputBuffer.Reset()
				outputBuffer.WriteString(trimmed)
				bufferMarkPos -= dropped
				if bufferMarkPos < 0 {
					bufferMarkPos = 0
				}
			}
			outputMu.Unlock()
			notifyOutput()
		}
		if readErr != nil {
			_ = s.cmd.Wait()
			s.mgr.removeSession(s.id, nil)
			return
		}
	}
}

func (s *subprocessSession) runAutomation(steps []gossh.ParsedCommand, outputMu *sync.Mutex, outputBuffer *strings.Builder, bufferMarkPos *int, outputChan chan struct{}) {
	time.Sleep(500 * time.Millisecond)

	markHere := func() {
		outputMu.Lock()
		*bufferMarkPos = outputBuffer.Len()
		outputMu.Unlock()
	}

	sinceMarkContains := func(needle string) bool {
		outputMu.Lock()
		defer outputMu.Unlock()
		full := outputBuffer.String()
		if *bufferMarkPos >= len(full) {
			return false
		}
		return strings.Contains(strings.ToLower(full[*bufferMarkPos:]), needle)
	}

	for _, pc := range steps {
		switch pc.Type {
		case gossh.CommandTypeSend:
			fmt.Fprintf(s.ptmx, "%s\r", pc.Value)
			time.Sleep(500 * time.Millisecond)
			markHere()

		case gossh.CommandTypeSendPass:
			pwd, err := s.mgr.passwords.Reveal(pc.Value)
			if err != nil {
				s.mgr.emitOutput(s.id, fmt.Sprintf("\r\n[go-ssh-ui] kayıtlı şifre bulunamadı: %s\r\n", pc.Value))
				return
			}
			fmt.Fprintf(s.ptmx, "%s\r", pwd)
			time.Sleep(800 * time.Millisecond)
			markHere()

		case gossh.CommandTypeWait:
			var seconds int
			if n, err := fmt.Sscanf(pc.Value, "%d", &seconds); err == nil && n == 1 && seconds > 0 {
				time.Sleep(time.Duration(seconds) * time.Second)
			}

		case gossh.CommandTypeExpect:
			expected := strings.ToLower(pc.Value)
			if sinceMarkContains(expected) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			timeout := time.After(30 * time.Second)
			ticker := time.NewTicker(50 * time.Millisecond)
			func() {
				defer ticker.Stop()
				for {
					select {
					case <-outputChan:
						if sinceMarkContains(expected) {
							return
						}
					case <-ticker.C:
						if sinceMarkContains(expected) {
							return
						}
					case <-timeout:
						return
					}
				}
			}()

		case gossh.CommandTypeInteract:
			return

		case gossh.CommandTypeExec:
			fmt.Fprintf(s.ptmx, "%s\r", pc.Value)
			time.Sleep(200 * time.Millisecond)
			markHere()
		}
	}
}

// envWithOverrides returns the current process's environment with the given
// keys replaced rather than merely appended - a plain append(os.Environ(), "K=v")
// can silently lose to an existing "K=..." earlier in the slice, since which
// duplicate wins when there are two is left to the C library's getenv(), not
// guaranteed to be "last one written".
func envWithOverrides(overrides map[string]string) []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		key, _, found := strings.Cut(kv, "=")
		if found {
			if _, overridden := overrides[key]; overridden {
				continue
			}
		}
		env = append(env, kv)
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}
