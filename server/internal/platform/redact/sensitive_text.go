package redact

import "regexp"

var sensitiveAssignment = regexp.MustCompile(`(?i)(\b(?:setup_token|access_token|refresh_token|token|secret|password|passwd|api_key|rkey|SESSDATA|bili_jct)\s*[:=]\s*)([^&\s"'<>;,]+)`)
var sensitiveHeader = regexp.MustCompile(`(?im)(\b(?:authorization|cookie|set-cookie)\s*[:=]\s*)([^\r\n]+)`)

// SensitiveText masks recognizable credentials even before their values are registered.
func SensitiveText(text string) string {
	text = sensitiveHeader.ReplaceAllString(text, "${1}"+placeholder)
	return sensitiveAssignment.ReplaceAllString(text, "${1}"+placeholder)
}
