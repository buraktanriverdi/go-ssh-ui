// Package scpfs implements go-ssh-ui's file manager: not SFTP (no
// github.com/pkg/sftp dependency - see PLAN.md, this was an explicit user
// preference), but the classic scp wire protocol, driven against a real
// remote `scp -f`/`scp -t` process. That process is reached over either
// sshengine transport (a subprocess `ssh ... -- 'scp -f path'` pipe, or a
// native ssh.Client exec channel), so this package only depends on a plain
// io.Reader+io.Writer, not on either engine directly.
package scpfs

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Transport is a bidirectional byte stream to a process speaking the remote
// side of the scp protocol.
type Transport struct {
	R io.Reader
	W io.Writer
}

const (
	statusOK    = 0
	statusWarn  = 1
	statusFatal = 2
)

func sendOK(w io.Writer) error {
	_, err := w.Write([]byte{statusOK})
	return err
}

// readStatus reads a single status byte. A non-zero byte means the peer is
// reporting a warning/fatal error, whose message follows as a
// newline-terminated string.
func readStatus(r *bufio.Reader) error {
	b, err := r.ReadByte()
	if err != nil {
		return err
	}
	if b == statusOK {
		return nil
	}
	msg, _ := r.ReadString('\n')
	kind := "uyarı"
	if b == statusFatal {
		kind = "hata"
	}
	return fmt.Errorf("scp %s: %s", kind, strings.TrimRight(msg, "\n"))
}

// ProgressFunc is called periodically during a transfer with bytes moved so
// far and the total (from the C control line), so a UI can show a real
// percentage - the classic scp protocol reports the size up front, unlike
// raw byte-stream copies where the total wouldn't otherwise be known.
type ProgressFunc func(done, total int64)

// FileInfo describes the single file a Download call retrieved.
type FileInfo struct {
	Name string
	Mode string // e.g. "0644"
	Size int64
}

// Download drives a remote `scp -f <path>` process (already running on the
// other end of t) and writes the one file it sends to w.
func Download(t Transport, w io.Writer, onProgress ProgressFunc) (FileInfo, error) {
	r := bufio.NewReader(t.R)

	// scp -f waits for an initial readiness ACK before sending anything.
	if err := sendOK(t.W); err != nil {
		return FileInfo{}, fmt.Errorf("scp -f başlatılamadı: %w", err)
	}

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return FileInfo{}, fmt.Errorf("scp -f'tan okunamadı: %w", err)
		}
		if line == "" {
			continue
		}

		switch line[0] {
		case 'T':
			// Timestamps (only sent if the remote scp used -p, which we
			// never request) - ack and move on to the real C line.
			if err := sendOK(t.W); err != nil {
				return FileInfo{}, err
			}
			continue

		case 'C':
			var mode uint32
			var size int64
			var name string
			if _, err := fmt.Sscanf(line, "C%o %d %s", &mode, &size, &name); err != nil {
				return FileInfo{}, fmt.Errorf("beklenmeyen scp kontrol satırı: %q", line)
			}
			if err := sendOK(t.W); err != nil {
				return FileInfo{}, err
			}

			written, copyErr := copyN(w, r, size, func(done int64) {
				if onProgress != nil {
					onProgress(done, size)
				}
			})
			if copyErr != nil {
				return FileInfo{}, fmt.Errorf("dosya içeriği okunamadı (%d/%d byte): %w", written, size, copyErr)
			}

			if err := readStatus(r); err != nil {
				return FileInfo{}, err
			}
			if err := sendOK(t.W); err != nil {
				return FileInfo{}, err
			}
			return FileInfo{Name: name, Mode: fmt.Sprintf("%04o", mode), Size: size}, nil

		case statusWarn, statusFatal:
			msg, _ := r.ReadString('\n')
			return FileInfo{}, fmt.Errorf("scp: %s", strings.TrimRight(msg, "\n"))

		default:
			return FileInfo{}, fmt.Errorf("beklenmeyen scp mesajı: %q", line)
		}
	}
}

// Upload drives a remote `scp -t <destination>` process (already running on
// the other end of t), sending it exactly one file read from r.
func Upload(t Transport, r io.Reader, name string, size int64, mode string, onProgress ProgressFunc) error {
	br := bufio.NewReader(t.R)

	if _, err := fmt.Fprintf(t.W, "C%s %d %s\n", mode, size, name); err != nil {
		return fmt.Errorf("scp -t'ye kontrol satırı gönderilemedi: %w", err)
	}
	if err := readStatus(br); err != nil {
		return err
	}

	written, err := copyN(writerFunc(func(p []byte) (int, error) {
		return t.W.Write(p)
	}), r, size, func(done int64) {
		if onProgress != nil {
			onProgress(done, size)
		}
	})
	if err != nil {
		return fmt.Errorf("dosya içeriği gönderilemedi (%d/%d byte): %w", written, size, err)
	}

	if err := sendOK(t.W); err != nil {
		return fmt.Errorf("bitiş sinyali gönderilemedi: %w", err)
	}
	return readStatus(br)
}

// RelayCopy drives a remote `scp -f` process on src and a remote `scp -t`
// process on dst at once, relaying the one file src sends directly into
// dst without buffering it locally - used for remote-to-remote copies,
// where composing the black-box Download/Upload functions doesn't work
// because dst's header can't be sent until src's has been read, and
// Download only reports that after the whole transfer already completed.
func RelayCopy(src, dst Transport, onProgress ProgressFunc) error {
	srcR := bufio.NewReader(src.R)
	if err := sendOK(src.W); err != nil {
		return fmt.Errorf("kaynak başlatılamadı: %w", err)
	}

	var mode string
	var size int64
	var name string
	for {
		line, err := srcR.ReadString('\n')
		if err != nil {
			return fmt.Errorf("kaynaktan okunamadı: %w", err)
		}
		if line == "" {
			continue
		}
		if line[0] == 'T' {
			if err := sendOK(src.W); err != nil {
				return err
			}
			continue
		}
		var m uint32
		if _, err := fmt.Sscanf(line, "C%o %d %s", &m, &size, &name); err != nil {
			return fmt.Errorf("beklenmeyen scp kontrol satırı: %q", line)
		}
		mode = fmt.Sprintf("%04o", m)
		break
	}
	if err := sendOK(src.W); err != nil {
		return err
	}

	dstR := bufio.NewReader(dst.R)
	if _, err := fmt.Fprintf(dst.W, "C%s %d %s\n", mode, size, name); err != nil {
		return fmt.Errorf("hedefe kontrol satırı gönderilemedi: %w", err)
	}
	if err := readStatus(dstR); err != nil {
		return err
	}

	written, err := copyN(dst.W, srcR, size, func(done int64) {
		if onProgress != nil {
			onProgress(done, size)
		}
	})
	if err != nil {
		return fmt.Errorf("kopyalama başarısız (%d/%d byte): %w", written, size, err)
	}

	if err := readStatus(srcR); err != nil {
		return err
	}
	if err := sendOK(src.W); err != nil {
		return err
	}
	if err := sendOK(dst.W); err != nil {
		return fmt.Errorf("bitiş sinyali gönderilemedi: %w", err)
	}
	return readStatus(dstR)
}

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// copyN copies exactly n bytes from src to dst, calling onChunk with the
// running total after each chunk - like io.CopyN, but with progress
// reporting built in instead of needing a wrapper writer per call site.
func copyN(dst io.Writer, src io.Reader, n int64, onChunk func(done int64)) (int64, error) {
	const chunkSize = 32 * 1024
	buf := make([]byte, chunkSize)
	var done int64
	for done < n {
		toRead := int64(chunkSize)
		if remaining := n - done; remaining < toRead {
			toRead = remaining
		}
		nr, err := io.ReadFull(src, buf[:toRead])
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			done += int64(nw)
			if werr != nil {
				return done, werr
			}
			if onChunk != nil {
				onChunk(done)
			}
		}
		if err != nil {
			return done, err
		}
	}
	return done, nil
}
