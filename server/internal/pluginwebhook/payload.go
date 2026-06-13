package pluginwebhook

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

func (s *Service) buildWebhookRawPayload(r *http.Request, route string, body []byte, include bool) any {
	if !include {
		return nil
	}

	payload := map[string]any{
		"route":        route,
		"method":       r.Method,
		"content_type": r.Header.Get("Content-Type"),
		"headers":      cloneWebhookHeaders(r.Header),
		"query":        cloneWebhookQuery(r.URL.Query()),
	}
	if len(body) == 0 {
		return payload
	}

	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.Contains(contentType, "application/json") {
		var decoded any
		if err := json.Unmarshal(body, &decoded); err == nil {
			payload["body_json"] = decoded
			return payload
		}
	}
	if utf8.Valid(body) {
		payload["body_text"] = string(body)
		return payload
	}
	payload["body_base64"] = base64.StdEncoding.EncodeToString(body)
	return payload
}

func cloneWebhookHeaders(headers http.Header) map[string]any {
	result := make(map[string]any, len(headers))
	for key, values := range headers {
		copied := make([]string, len(values))
		copy(copied, values)
		result[key] = copied
	}
	return result
}

func cloneWebhookQuery(values url.Values) map[string]any {
	result := make(map[string]any, len(values))
	for key, items := range values {
		copied := make([]string, len(items))
		copy(copied, items)
		result[key] = copied
	}
	return result
}
