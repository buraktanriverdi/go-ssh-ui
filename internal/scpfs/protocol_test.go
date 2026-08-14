package scpfs

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// These tests drive the real local `scp` binary as the remote peer (in -f
// and -t server modes) over plain OS pipes - no network/SSH involved, but
// it's the actual scp implementation on the other end, so a pass here means
// the protocol implementation is byte-correct against a real interop
// partner, not just internally consistent.

func TestDownloadAgainstRealScp(t *testing.T) {
	if _, err := exec.LookPath("scp"); err != nil {
		t.Skip("scp not available")
	}

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.txt")
	content := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 500) // >20KB, exercises chunked copy
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("scp", "-f", srcPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	var got bytes.Buffer
	var progressCalls int
	info, err := Download(Transport{R: stdout, W: stdin}, &got, func(done, total int64) {
		progressCalls++
		if done > total {
			t.Errorf("progress done %d exceeds total %d", done, total)
		}
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	_ = cmd.Wait()

	if info.Name != "source.txt" {
		t.Errorf("name = %q, want source.txt", info.Name)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", info.Size, len(content))
	}
	if !bytes.Equal(got.Bytes(), content) {
		t.Fatalf("downloaded content mismatch: got %d bytes, want %d bytes", got.Len(), len(content))
	}
	if progressCalls == 0 {
		t.Error("expected at least one progress callback")
	}
}

func TestUploadAgainstRealScp(t *testing.T) {
	if _, err := exec.LookPath("scp"); err != nil {
		t.Skip("scp not available")
	}

	dir := t.TempDir()
	content := bytes.Repeat([]byte("upload roundtrip test data\n"), 500)

	cmd := exec.Command("scp", "-t", dir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	var progressCalls int
	err = Upload(Transport{R: stdout, W: stdin}, bytes.NewReader(content), "uploaded.txt", int64(len(content)), "0644", func(done, total int64) {
		progressCalls++
		if done > total {
			t.Errorf("progress done %d exceeds total %d", done, total)
		}
	})
	_ = stdin.Close()
	waitErr := cmd.Wait()
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if waitErr != nil {
		t.Fatalf("scp -t exited with error: %v", waitErr)
	}

	got, err := os.ReadFile(filepath.Join(dir, "uploaded.txt"))
	if err != nil {
		t.Fatalf("reading uploaded file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("uploaded content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
	if progressCalls == 0 {
		t.Error("expected at least one progress callback")
	}
}

func TestRelayCopyAgainstRealScp(t *testing.T) {
	if _, err := exec.LookPath("scp"); err != nil {
		t.Skip("scp not available")
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "relay-source.txt")
	content := bytes.Repeat([]byte("remote-to-remote relay test\n"), 500)
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	srcCmd := exec.Command("scp", "-f", srcPath)
	srcStdin, err := srcCmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	srcStdout, err := srcCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := srcCmd.Start(); err != nil {
		t.Fatal(err)
	}

	dstCmd := exec.Command("scp", "-t", dstDir)
	dstStdin, err := dstCmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	dstStdout, err := dstCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := dstCmd.Start(); err != nil {
		t.Fatal(err)
	}

	var progressCalls int
	err = RelayCopy(
		Transport{R: srcStdout, W: srcStdin},
		Transport{R: dstStdout, W: dstStdin},
		func(done, total int64) {
			progressCalls++
			if done > total {
				t.Errorf("progress done %d exceeds total %d", done, total)
			}
		},
	)
	if err != nil {
		t.Fatalf("RelayCopy: %v", err)
	}
	_ = srcCmd.Wait()
	_ = dstStdin.Close()
	_ = dstCmd.Wait()

	got, err := os.ReadFile(filepath.Join(dstDir, "relay-source.txt"))
	if err != nil {
		t.Fatalf("reading relayed file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("relayed content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
	if progressCalls == 0 {
		t.Error("expected at least one progress callback")
	}
}
