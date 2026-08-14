package scpfs

import (
	"fmt"
	"io"
	"path"
	"strings"

	"go-ssh/config"
	"go-ssh-ui/internal/sshengine"
)

// Service is the file-manager backend: directory listing and scp transfers
// driven over whichever sshengine transport a host supports (see
// sshengine.CanRunOneOff, OpenExec, RunRemoteCommand).
type Service struct {
	mgr *sshengine.Manager
}

func NewService(mgr *sshengine.Manager) *Service {
	return &Service{mgr: mgr}
}

// List returns the contents of dir ("." if empty). LC_ALL=C keeps the
// output in a fixed, parseable format regardless of the remote server's
// own default locale (non-interactive ssh commands don't inherit the
// local shell's locale env vars, but forcing it explicitly is cheap
// insurance rather than an assumption).
func (s *Service) List(host config.Host, dir string) ([]Entry, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	out, err := s.mgr.RunRemoteCommand(host, "LC_ALL=C ls -la -- "+shellQuote(dir))
	if err != nil {
		return nil, err
	}
	return ParseLsLa(string(out)), nil
}

// Mkdir creates a directory, including any missing parents.
func (s *Service) Mkdir(host config.Host, dir string) error {
	_, err := s.mgr.RunRemoteCommand(host, "mkdir -p -- "+shellQuote(dir))
	return err
}

// Delete removes a file or (recursively) a directory.
func (s *Service) Delete(host config.Host, target string) error {
	_, err := s.mgr.RunRemoteCommand(host, "rm -rf -- "+shellQuote(target))
	return err
}

// Rename moves/renames a file or directory.
func (s *Service) Rename(host config.Host, from, to string) error {
	_, err := s.mgr.RunRemoteCommand(host, "mv -- "+shellQuote(from)+" "+shellQuote(to))
	return err
}

// Download streams remotePath's contents to w, driving a remote `scp -f`.
func (s *Service) Download(host config.Host, remotePath string, w io.Writer, onProgress ProgressFunc) (FileInfo, error) {
	stream, err := s.mgr.OpenExec(host, "scp -f -- "+shellQuote(remotePath))
	if err != nil {
		return FileInfo{}, err
	}
	defer stream.Close()
	return Download(Transport{R: stream, W: stream}, w, onProgress)
}

// Upload streams r (size bytes, POSIX mode e.g. "0644") to remoteDir/name,
// driving a remote `scp -t`.
func (s *Service) Upload(host config.Host, remoteDir, name string, r io.Reader, size int64, mode string, onProgress ProgressFunc) error {
	stream, err := s.mgr.OpenExec(host, "scp -t -- "+shellQuote(remoteDir))
	if err != nil {
		return err
	}
	defer stream.Close()
	return Upload(Transport{R: stream, W: stream}, r, name, size, mode, onProgress)
}

// CopyRemoteToRemote copies remotePath on srcHost into dstDir on dstHost by
// driving both scp protocol sessions at once and relaying the file bytes
// directly from one to the other (see RelayCopy) - the standard trick (what
// OpenSSH's own `scp -3` does under the hood) since the scp protocol has no
// server-to-server copy primitive.
func (s *Service) CopyRemoteToRemote(srcHost config.Host, remotePath string, dstHost config.Host, dstDir string, onProgress ProgressFunc) error {
	src, err := s.mgr.OpenExec(srcHost, "scp -f -- "+shellQuote(remotePath))
	if err != nil {
		return fmt.Errorf("kaynağa bağlanılamadı: %w", err)
	}
	defer src.Close()

	dst, err := s.mgr.OpenExec(dstHost, "scp -t -- "+shellQuote(dstDir))
	if err != nil {
		return fmt.Errorf("hedefe bağlanılamadı: %w", err)
	}
	defer dst.Close()

	return RelayCopy(Transport{R: src, W: src}, Transport{R: dst, W: dst}, onProgress)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// JoinRemote joins remote path segments with POSIX "/" semantics
// regardless of the local OS (path/filepath would use "\" on Windows).
func JoinRemote(elem ...string) string {
	return path.Join(elem...)
}
