package scpfs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListLocal lists a local directory for the file manager's local pane - no
// SSH involved, just the local filesystem, kept in the same Entry shape as
// ListDir's remote results so the frontend can use one component for both
// panes.
func ListLocal(dir string) ([]Entry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("dizin okunamadı: %w", err)
	}

	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			continue
		}
		target := ""
		isLink := info.Mode()&os.ModeSymlink != 0
		if isLink {
			if resolved, err := os.Readlink(filepath.Join(dir, item.Name())); err == nil {
				target = resolved
			}
		}
		entries = append(entries, Entry{
			Name:       item.Name(),
			IsDir:      item.IsDir(),
			IsLink:     isLink,
			LinkTarget: target,
			Size:       info.Size(),
			Mode:       info.Mode().String(),
			ModTime:    info.ModTime().Format("Jan 2 15:04"),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}
