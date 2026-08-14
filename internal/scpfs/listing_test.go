package scpfs

import "testing"

func TestParseLsLaMacOS(t *testing.T) {
	// Real `ls -la` output shape from macOS (BSD ls).
	out := "total 24\n" +
		"drwxr-xr-x   5 burak  staff   160 Aug 13 22:46 .\n" +
		"drwxr-xr-x  20 burak  staff   640 Aug 13 09:00 ..\n" +
		"-rw-r--r--   1 burak  staff  1234 Aug 12 10:15 report.txt\n" +
		"drwxr-xr-x   3 burak  staff    96 Aug 10 2023 old-logs\n" +
		"lrwxr-xr-x   1 burak  staff    11 Aug  1 08:00 current -> report.txt\n"

	entries := ParseLsLa(out)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (excluding . and ..), got %d: %+v", len(entries), entries)
	}

	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}

	report, ok := byName["report.txt"]
	if !ok || report.IsDir || report.Size != 1234 {
		t.Errorf("report.txt parsed incorrectly: %+v (ok=%v)", report, ok)
	}

	dir, ok := byName["old-logs"]
	if !ok || !dir.IsDir {
		t.Errorf("old-logs parsed incorrectly: %+v (ok=%v)", dir, ok)
	}

	link, ok := byName["current"]
	if !ok || !link.IsLink || link.LinkTarget != "report.txt" {
		t.Errorf("current symlink parsed incorrectly: %+v (ok=%v)", link, ok)
	}

	// Directories should sort before files, alphabetically within each group.
	if entries[0].Name != "old-logs" {
		t.Errorf("expected old-logs first (dirs before files), got order: %v", names(entries))
	}
}

func TestParseLsLaMacOSExtendedAttributes(t *testing.T) {
	// BSD ls appends a marker after the mode bits for files with extended
	// attributes (@) or ACLs (+) - a real, commonly-hit case on macOS.
	out := "total 8\n" +
		"drwxr-xr-x@  5 burak  staff  160 Aug 13 22:46 .\n" +
		"-rw-r--r--@  1 burak  staff  100 Aug 12 10:15 note.txt\n" +
		"-rw-r--r--+  1 burak  staff  200 Aug 12 10:16 acl-file.txt\n"

	entries := ParseLsLa(out)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Name != "acl-file.txt" && entries[1].Name != "acl-file.txt" {
		t.Errorf("acl-file.txt not parsed: %+v", entries)
	}
}

func TestParseLsLaLinux(t *testing.T) {
	// Real `ls -la` output shape from a typical GNU/Linux box.
	out := "total 20\n" +
		"drwxr-xr-x 2 root root 4096 Aug 13 22:46 .\n" +
		"drwxr-xr-x 3 root root 4096 Aug 13 09:00 ..\n" +
		"-rwxr-xr-x 1 root root  512 Aug 12 10:15 deploy.sh\n"

	entries := ParseLsLa(out)
	if len(entries) != 1 || entries[0].Name != "deploy.sh" || entries[0].Size != 512 {
		t.Fatalf("unexpected parse result: %+v", entries)
	}
}

func names(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name
	}
	return out
}
