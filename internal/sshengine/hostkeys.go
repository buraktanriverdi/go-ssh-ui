package sshengine

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

// hostKeyCallback checks server host keys against the user's real
// ~/.ssh/known_hosts (so it stays in sync with whatever the system `ssh`
// binary and other tools already trust), asking the frontend via the
// prompt broker on trust-on-first-use for a brand-new host, or on a
// changed key (possible MITM - the callback still requires an explicit
// "trust" answer, and never silently rewrites the changed entry).
func (m *Manager) hostKeyCallback(sessionID string) (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	khPath := filepath.Join(home, ".ssh", "known_hosts")
	if _, statErr := os.Stat(khPath); os.IsNotExist(statErr) {
		if mkErr := os.MkdirAll(filepath.Dir(khPath), 0700); mkErr == nil {
			_ = os.WriteFile(khPath, nil, 0600)
		}
	}

	db, err := knownhosts.NewDB(khPath)
	if err != nil {
		return nil, fmt.Errorf("known_hosts okunamadı: %w", err)
	}
	base := db.HostKeyCallback()

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := base(hostname, remote, key)
		if err == nil {
			return nil
		}

		changed := knownhosts.IsHostKeyChanged(err)
		unknown := knownhosts.IsHostUnknown(err)
		if !changed && !unknown {
			return err
		}

		answer, askErr := m.broker.Ask(Prompt{
			SessionID:   sessionID,
			Kind:        PromptHostKey,
			Hostname:    hostname,
			Fingerprint: ssh.FingerprintSHA256(key),
			KeyType:     key.Type(),
			IsChanged:   changed,
		})
		if askErr != nil || !answer.Trusted {
			if askErr != nil {
				return askErr
			}
			return fmt.Errorf("host anahtarı reddedildi")
		}

		if unknown {
			// Only append brand-new hosts. A changed key is trusted for this
			// session only - appending would leave both the old (possibly
			// compromised) and new lines in the file, which is worse than
			// asking again next time.
			if f, openErr := os.OpenFile(khPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); openErr == nil {
				_ = knownhosts.WriteKnownHost(f, hostname, remote, key)
				_ = f.Close()
			}
		}
		return nil
	}, nil
}
