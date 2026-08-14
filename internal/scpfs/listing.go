package scpfs

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Entry is one row of a remote directory listing, parsed from `ls -la`
// output - the SCP protocol itself has no listing operation (see
// PLAN.md §5), so this is the only source of directory contents.
type Entry struct {
	Name       string `json:"name"`
	IsDir      bool   `json:"isDir"`
	IsLink     bool   `json:"isLink"`
	LinkTarget string `json:"linkTarget,omitempty"`
	Size       int64  `json:"size"`
	Mode       string `json:"mode"`
	ModTime    string `json:"modTime"`
}

// lsLine matches one `ls -la` output line on both GNU and BSD (macOS) ls:
// mode(10 chars, optionally +1 for BSD's ACL/xattr marker: @ + or .) links
// owner group size "Mon DD HH:MM"|"Mon DD YYYY" name. The remote command
// this is meant to parse always forces `LC_ALL=C` (see scpfs service code)
// specifically so the month name is a predictable "Jan".."Dec" regardless
// of the remote server's own locale.
var lsLine = regexp.MustCompile(`^(\S{10})[@+.]?\s+(\d+)\s+(\S+)\s+(\S+)\s+(\d+)\s+(\w{3}\s+\d{1,2}\s+(?:\d{2}:\d{2}|\d{4}))\s+(.+)$`)

// ParseLsLa parses the combined output of `ls -la` into entries, dropping
// the "total N" header line and "." / "..". Lines that don't match the
// expected column layout are silently skipped rather than failing the
// whole listing - remote `ls` implementations vary enough (locale-specific
// month names, unusual filesystems) that being lenient beats erroring out
// on one odd line.
func ParseLsLa(output string) []Entry {
	var entries []Entry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		m := lsLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		mode := m[1]
		size, _ := strconv.ParseInt(m[5], 10, 64)
		name := m[7]

		isLink := mode[0] == 'l'
		var target string
		if isLink {
			if idx := strings.Index(name, " -> "); idx >= 0 {
				target = name[idx+4:]
				name = name[:idx]
			}
		}
		if name == "." || name == ".." {
			continue
		}

		entries = append(entries, Entry{
			Name:       name,
			IsDir:      mode[0] == 'd',
			IsLink:     isLink,
			LinkTarget: target,
			Size:       size,
			Mode:       mode,
			ModTime:    m[6],
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}
