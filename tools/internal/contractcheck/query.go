package contractcheck

import (
	"net/url"
	"strings"
)

// Python parse_qs uses '&' as the separator and preserves semicolons in values.
// net/url.ParseQuery drops fields with unescaped semicolons instead.
func requestQuery(raw string) url.Values {
	values := url.Values{}
	for _, field := range strings.Split(raw, "&") {
		if field == "" {
			continue
		}
		key, value, _ := strings.Cut(field, "=")
		key, value = queryUnescape(key), queryUnescape(value)
		values[key] = append(values[key], value)
	}
	return values
}
func queryUnescape(raw string) string {
	var escaped strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '%' && (i+2 >= len(raw) || !hexByte(raw[i+1]) || !hexByte(raw[i+2])) {
			escaped.WriteString("%25")
		} else {
			escaped.WriteByte(raw[i])
		}
	}
	value, _ := url.QueryUnescape(escaped.String())
	return strings.ToValidUTF8(value, "\uFFFD")
}
func hexByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
