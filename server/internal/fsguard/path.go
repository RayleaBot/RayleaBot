// Package fsguard provides lexical path checks shared by filesystem boundaries.
// Callers must separately enforce ownership, permitted entries and symlink rules.
package fsguard

import (
	"errors"
	"path"
	"path/filepath"
	"strings"
)

func WithinRoot(root, candidate string) bool {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absoluteCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteCandidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// ArchivePath normalizes a slash-separated archive name without allowing it to
// escape its root. Windows validation is independent of the host platform.
func ArchivePath(name string, windows bool) (string, error) {
	if name == "" || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return "", errors.New("archive entry must be a relative slash path")
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("archive entry escapes its root")
	}
	if windows {
		for _, part := range strings.Split(clean, "/") {
			if !WindowsSegment(part) {
				return "", errors.New("archive entry contains an unsafe Windows name")
			}
		}
	}
	return clean, nil
}

func WindowsSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `<>:"|?*\/`) || strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
		return false
	}
	for _, character := range segment {
		if character < 0x20 {
			return false
		}
	}
	base, _, _ := strings.Cut(segment, ".")
	base = strings.ToUpper(strings.TrimRight(base, " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return false
	}
	if strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT") {
		suffix := base[3:]
		if len(suffix) == 1 && suffix[0] >= '1' && suffix[0] <= '9' {
			return false
		}
		switch suffix {
		case "¹", "²", "³":
			return false
		}
	}
	return true
}
