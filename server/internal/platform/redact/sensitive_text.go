package redact

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var sensitiveAssignment = newSensitiveTextPattern([]string{
	"setup_token", "access_token", "refresh_token", "token", "secret", "password", "passwd", "api_key", "rkey", "sessdata", "bili_jct",
	"ltoken", "ltoken_v2", "cookie_token", "cookie_token_v2", "stoken", "stoken_v2", "login_ticket", "authkey", "game_token", "combo_token",
}, `[^&\s"'<>;,]+`)
var sensitiveHeader = newSensitiveTextPattern([]string{"authorization", "cookie", "set-cookie"}, `[^\r\n]+`)

type sensitiveTextPattern struct {
	names      []string
	expression *regexp.Regexp
}

func newSensitiveTextPattern(names []string, valuePattern string) sensitiveTextPattern {
	escaped := make([]string, len(names))
	for index, name := range names {
		escaped[index] = regexp.QuoteMeta(name)
	}
	return sensitiveTextPattern{names: names, expression: regexp.MustCompile(`(?i)^((?:` + strings.Join(escaped, "|") + `)\s*[:=]\s*)(` + valuePattern + `)`)}
}

// SensitiveText masks recognizable credentials even before their values are registered.
func SensitiveText(text string) string {
	if !strings.ContainsAny(text, ":=") {
		return text
	}
	text = sensitiveHeader.replace(text)
	return sensitiveAssignment.replace(text)
}

func (pattern sensitiveTextPattern) replace(text string) string {
	var output strings.Builder
	copied, search := 0, 0
	for search < len(text) {
		separator := strings.IndexAny(text[search:], ":=")
		if separator < 0 {
			break
		}
		separator += search
		start := pattern.keyStart(text, separator, search)
		if start < 0 {
			search = separator + 1
			continue
		}
		// The anchored expression retains regexp's greedy whitespace and value
		// matching without searching ordinary message text for every key.
		match := pattern.expression.FindStringSubmatchIndex(text[start:])
		if match == nil {
			search = separator + 1
			continue
		}
		if text[start+match[3]:start+match[1]] == placeholder {
			search = start + match[1]
			continue
		}
		if copied == 0 {
			output.Grow(len(text) - (match[1] - match[3]) + len(placeholder))
		}
		output.WriteString(text[copied : start+match[3]])
		output.WriteString(placeholder)
		copied = start + match[1]
		search = copied
	}
	if copied == 0 {
		return text
	}
	output.WriteString(text[copied:])
	return output.String()
}

func (pattern sensitiveTextPattern) keyStart(text string, separator, minimum int) int {
	end := separator
	for end > minimum && regexpSpace(text[end-1]) {
		end--
	}
	earliest := -1
	for _, name := range pattern.names {
		start := end
		matched := true
		for index := len(name) - 1; index >= 0; index-- {
			if start <= minimum {
				matched = false
				break
			}
			actual, size := utf8.DecodeLastRuneInString(text[:start])
			start -= size
			if !matchesFoldedKeyRune(actual, name[index]) {
				matched = false
				break
			}
		}
		// Go regexp's \b uses ASCII word characters even under (?i).
		// A folded non-ASCII first rune can therefore follow an ASCII word.
		if matched && (start > 0 && regexpWord(text[start-1])) != regexpWord(text[start]) && (earliest < 0 || start < earliest) {
			earliest = start
		}
	}
	return earliest
}

func matchesFoldedKeyRune(actual rune, expected byte) bool {
	if actual == rune(expected) || expected >= 'a' && expected <= 'z' && actual == rune(expected-'a'+'A') {
		return true
	}
	if actual < utf8.RuneSelf {
		return false
	}
	for folded := unicode.SimpleFold(actual); folded != actual; folded = unicode.SimpleFold(folded) {
		if folded == rune(expected) {
			return true
		}
	}
	return false
}

func regexpSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f'
}

func regexpWord(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}
