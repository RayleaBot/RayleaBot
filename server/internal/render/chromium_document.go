package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func writeTemporaryRenderDocument(html, baseURL string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "rayleabot-render-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	documentPath := filepath.Join(dir, "document.html")
	if err := os.WriteFile(documentPath, []byte(htmlWithBaseURL(html, baseURL)), 0o600); err != nil {
		cleanup()
		return "", nil, err
	}
	return fileURL(documentPath), cleanup, nil
}

var headOpenPattern = regexp.MustCompile(`(?i)<head(\s[^>]*)?>`)

func htmlWithBaseURL(html, baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" || strings.Contains(strings.ToLower(html), "<base ") {
		return html
	}

	baseElement := `<base href="` + strings.ReplaceAll(baseURL, `"`, "%22") + `">`
	if location := headOpenPattern.FindStringIndex(html); location != nil {
		return html[:location[1]] + baseElement + html[location[1]:]
	}
	return baseElement + html
}
