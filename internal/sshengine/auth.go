package sshengine

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"go-ssh/config"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func expandHome(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// authMethods builds the ssh.AuthMethod list for host. It always appends a
// keyboard-interactive method: if the server only asks a single question and
// host is in password mode, it auto-answers with the stored password
// (the common "Password:" re-prompt case); otherwise it bridges every
// question to the frontend, which covers 2FA/OTP and any other prompt shape.
func (m *Manager) authMethods(sessionID string, host config.Host) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	var storedPassword string

	switch host.AuthMethod {
	case "password":
		pw, err := m.passwords.Reveal(host.PasswordID)
		if err != nil {
			return nil, fmt.Errorf("kayıtlı şifre bulunamadı (%s): %w", host.PasswordID, err)
		}
		storedPassword = pw
		methods = append(methods, ssh.Password(pw))

	case "key":
		signer, err := m.loadKeySigner(sessionID, host.IdentityFile)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))

	case "agent":
		signers, err := agentSigners()
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeysCallback(signers))

	default:
		return nil, fmt.Errorf("host için kimlik doğrulama yöntemi seçilmemiş")
	}

	methods = append(methods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
		if len(questions) == 1 && storedPassword != "" {
			return []string{storedPassword}, nil
		}
		answer, err := m.broker.Ask(Prompt{
			SessionID:   sessionID,
			Kind:        PromptKeyboardInteractive,
			Name:        name,
			Instruction: instruction,
			Questions:   questions,
			Echo:        echos,
		})
		if err != nil {
			return nil, err
		}
		return answer.Values, nil
	}))

	return methods, nil
}

// loadKeySigner reads and parses a private key file, asking the frontend for
// its passphrase (via the same prompt bridge as 2FA) if it's encrypted.
func (m *Manager) loadKeySigner(sessionID, path string) (ssh.Signer, error) {
	if path == "" {
		return nil, fmt.Errorf("anahtar dosyası belirtilmemiş")
	}
	path = expandHome(path)
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("anahtar dosyası okunamadı (%s): %w", path, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err == nil {
		return signer, nil
	}

	var passErr *ssh.PassphraseMissingError
	if !errors.As(err, &passErr) {
		return nil, fmt.Errorf("anahtar ayrıştırılamadı: %w", err)
	}

	answer, askErr := m.broker.Ask(Prompt{
		SessionID:   sessionID,
		Kind:        PromptKeyPassphrase,
		Name:        "Anahtar parolası",
		Instruction: path,
		Questions:   []string{"Parola"},
		Echo:        []bool{false},
	})
	if askErr != nil {
		return nil, askErr
	}
	if len(answer.Values) == 0 || answer.Values[0] == "" {
		return nil, fmt.Errorf("parola girilmedi")
	}
	return ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(answer.Values[0]))
}

func agentSigners() (func() ([]ssh.Signer, error), error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK tanımlı değil (ssh-agent çalışmıyor olabilir)")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("ssh-agent'a bağlanılamadı: %w", err)
	}
	ag := agent.NewClient(conn)
	return ag.Signers, nil
}
