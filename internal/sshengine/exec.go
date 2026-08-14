package sshengine

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"go-ssh/config"
	gossh "go-ssh/ssh"

	"github.com/google/uuid"
)

// simpleSSHCommand matches a legacy Host.Command that is *only* an ssh
// invocation - flags then a bare destination - with no embedded remote
// command already baked in (e.g. `ssh -tt bastion "ssh -tt inner"` or
// `ssh host 'cd /var/www && exec bash'` do NOT match). That distinction
// matters because CanRunOneOff commands work by appending our own remote
// command to the end of Host.Command; if the command already carries a
// remote command of its own, appending another would either be silently
// ignored or produce nonsense, so those hosts are conservatively treated as
// unsupported for one-off exec instead of risking either outcome.
var simpleSSHCommand = regexp.MustCompile(`^ssh(\s+-[A-Za-z]+(\s+\S+)?)*\s+([\w.\-]+@)?[\w.\-\[\]:]+\s*$`)

// CanRunOneOff reports whether host supports one-off, non-interactive
// commands - used by the file manager for directory listing and scp
// transfers. True for structured (native engine) hosts, and for legacy
// hosts whose Command is nothing more than a bare `ssh` invocation. False
// for anything using multi-step Commands (SEND/SENDPASS/WAIT/EXPECT
// automation, dzdo/sudo escalation) - a one-off `ssh host cmd` call would
// skip all of that scripting and either fail or run as the wrong user.
func CanRunOneOff(host config.Host) bool {
	if host.HostAddr != "" {
		return true
	}
	if len(host.Commands) != 0 {
		return false
	}
	return simpleSSHCommand.MatchString(strings.TrimSpace(host.Command))
}

// RunRemoteCommand runs remoteCmd non-interactively on host and returns its
// combined stdout+stderr once it exits. For listing/mkdir/rm/mv - not for
// scp transfers, which need a live streaming Transport (see OpenExec).
func (m *Manager) RunRemoteCommand(host config.Host, remoteCmd string) ([]byte, error) {
	if !CanRunOneOff(host) {
		return nil, fmt.Errorf("bu host için dosya yöneticisi henüz desteklenmiyor - önce bir terminal sekmesinden bağlan")
	}

	if host.HostAddr != "" {
		sessionID := uuid.NewString()
		client, clientKey, err := m.dialChain(sessionID, host)
		if err != nil {
			return nil, err
		}
		defer m.releaseClient(clientKey)

		sess, err := client.NewSession()
		if err != nil {
			return nil, fmt.Errorf("oturum açılamadı: %w", err)
		}
		defer sess.Close()

		out, err := sess.CombinedOutput(remoteCmd)
		if err != nil {
			return out, fmt.Errorf("komut başarısız: %w", err)
		}
		return out, nil
	}

	full, err := appendRemoteCommand(host.Command, remoteCmd)
	if err != nil {
		return nil, err
	}
	shell := shellPath()
	out, err := exec.Command(shell, "-c", full).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("komut başarısız: %w", err)
	}
	return out, nil
}

// execStream adapts either a native ssh.Session or a local *exec.Cmd's
// stdio pipes to a single io.ReadWriteCloser, so scpfs's scp -f/-t protocol
// code doesn't need to know which engine backs a host. Unlike
// RunRemoteCommand, this deliberately does NOT merge stderr into the
// stream, since that would corrupt the binary scp protocol on the other
// end of it.
type execStream struct {
	io.Reader
	io.Writer
	closeFn func() error
}

func (s *execStream) Close() error { return s.closeFn() }

// OpenExec opens a one-off, non-interactive command channel to host - not a
// shell/PTY, just "run this command and give me its live stdio" - used to
// drive a remote `scp -f`/`scp -t` process.
func (m *Manager) OpenExec(host config.Host, remoteCmd string) (io.ReadWriteCloser, error) {
	if !CanRunOneOff(host) {
		return nil, fmt.Errorf("bu host için dosya yöneticisi henüz desteklenmiyor - önce bir terminal sekmesinden bağlan")
	}
	if host.HostAddr != "" {
		return m.openExecNative(host, remoteCmd)
	}
	return m.openExecSubprocess(host, remoteCmd)
}

func (m *Manager) openExecNative(host config.Host, remoteCmd string) (io.ReadWriteCloser, error) {
	sessionID := uuid.NewString()
	client, clientKey, err := m.dialChain(sessionID, host)
	if err != nil {
		return nil, err
	}

	sess, err := client.NewSession()
	if err != nil {
		m.releaseClient(clientKey)
		return nil, fmt.Errorf("oturum açılamadı: %w", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		m.releaseClient(clientKey)
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		m.releaseClient(clientKey)
		return nil, err
	}
	if err := sess.Start(remoteCmd); err != nil {
		sess.Close()
		m.releaseClient(clientKey)
		return nil, fmt.Errorf("komut başlatılamadı: %w", err)
	}

	closed := false
	return &execStream{
		Reader: stdout,
		Writer: stdin,
		closeFn: func() error {
			if closed {
				return nil
			}
			closed = true
			err := sess.Close()
			m.releaseClient(clientKey)
			return err
		},
	}, nil
}

func (m *Manager) openExecSubprocess(host config.Host, remoteCmd string) (io.ReadWriteCloser, error) {
	full, err := appendRemoteCommand(host.Command, remoteCmd)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(shellPath(), "-c", full)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ssh başlatılamadı: %w", err)
	}

	closed := false
	return &execStream{
		Reader: stdout,
		Writer: stdin,
		closeFn: func() error {
			if closed {
				return nil
			}
			closed = true
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return cmd.Wait()
		},
	}, nil
}

// appendRemoteCommand validates that command is a bare `ssh` invocation
// (see simpleSSHCommand) and appends remoteCmd to it, single-quoted the
// same way go-ssh's own multi-hop command builder does.
func appendRemoteCommand(command, remoteCmd string) (string, error) {
	command = strings.TrimSpace(command)
	if !simpleSSHCommand.MatchString(command) {
		return "", fmt.Errorf("bu host için dosya yöneticisi henüz desteklenmiyor - önce bir terminal sekmesinden bağlan")
	}
	command = gossh.EnsureSSHAutoAcceptHostKeys(command)
	escaped := strings.ReplaceAll(remoteCmd, "'", `'"'"'`)
	return command + " '" + escaped + "'", nil
}

func shellPath() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash"
}
