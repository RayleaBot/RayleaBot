package adapter

import "strings"

// splitCQParams splits CQ parameters respecting that values may contain
// escaped commas.
func splitCQParams(s string) []string {
	var params []string
	var current strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == ',' {
			params = append(params, current.String())
			current.Reset()
			i++
			continue
		}
		current.WriteByte(s[i])
		i++
	}
	if current.Len() > 0 {
		params = append(params, current.String())
	}
	return params
}

// unescapeCQ reverses OneBot11 CQ code escape sequences.
func unescapeCQ(s string) string {
	s = strings.ReplaceAll(s, "&#44;", ",")
	s = strings.ReplaceAll(s, "&#91;", "[")
	s = strings.ReplaceAll(s, "&#93;", "]")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}
