package textsafe

import (
	"strings"
	"unicode/utf8"
)

// SanitizeString removes unsafe control characters from user-controlled text
// while preserving ordinary Unicode content.
func SanitizeString(value string) string {
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastCopy := 0
	modified := false

	for index := 0; index < len(value); {
		r, size := utf8.DecodeRuneInString(value[index:])

		replacement, shouldReplace, shouldDrop := sanitizeRune(r, size)
		if !shouldReplace && !shouldDrop {
			index += size
			continue
		}

		if !modified {
			modified = true
			builder.Grow(len(value))
		}
		if lastCopy < index {
			builder.WriteString(value[lastCopy:index])
		}
		if shouldReplace {
			builder.WriteString(replacement)
		}

		index += size
		lastCopy = index
	}

	if !modified {
		return value
	}
	if lastCopy < len(value) {
		builder.WriteString(value[lastCopy:])
	}

	return builder.String()
}

func sanitizeRune(r rune, size int) (replacement string, shouldReplace bool, shouldDrop bool) {
	if r == utf8.RuneError && size == 1 {
		return "", false, true
	}

	switch {
	case r == '\u2028' || r == '\u2029':
		return "\n", true, false
	case r == '\ufeff':
		return "", false, true
	case r == '\u061c' || r == '\u200e' || r == '\u200f':
		return "", false, true
	case r >= '\u202a' && r <= '\u202e':
		return "", false, true
	case r >= '\u2066' && r <= '\u206f':
		return "", false, true
	case r >= 0x00 && r <= 0x1f:
		if r == '\t' || r == '\n' || r == '\r' {
			return "", false, false
		}
		return "", false, true
	case r >= 0x7f && r <= 0x9f:
		return "", false, true
	default:
		return "", false, false
	}
}

// SanitizeAny recursively sanitizes all string values inside the provided
// value and returns a deep-cloned result for map/slice inputs.
func SanitizeAny(value any) any {
	switch typed := value.(type) {
	case string:
		return SanitizeString(typed)
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, inner := range typed {
			cloned[key] = SanitizeAny(inner)
		}
		return cloned
	case []any:
		cloned := make([]any, 0, len(typed))
		for _, inner := range typed {
			cloned = append(cloned, SanitizeAny(inner))
		}
		return cloned
	case []string:
		cloned := make([]string, 0, len(typed))
		for _, inner := range typed {
			cloned = append(cloned, SanitizeString(inner))
		}
		return cloned
	default:
		return value
	}
}

// TruncateRunes trims a string by rune count without splitting multi-byte
// characters.
func TruncateRunes(value string, limit int, suffix string) string {
	if limit <= 0 {
		return ""
	}

	count := 0
	for index := range value {
		if count == limit {
			return value[:index] + suffix
		}
		count++
	}

	return value
}
