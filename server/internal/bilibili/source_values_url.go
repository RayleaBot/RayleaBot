package bilibili

import (
	"bytes"
	"io"
	"net/url"
	"strings"
)

func normalizeURL(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "//") {
		return "https:" + text
	}
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return text
	}
	return text
}

func formBody(values url.Values) io.Reader {
	return bytes.NewBufferString(values.Encode())
}
