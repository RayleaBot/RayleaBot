package httpclient

import (
	"net/url"
	"strings"
)

func (c *Client) validateRequest(req Request) (*url.URL, string, []byte, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, "", nil, ErrInvalidRequest
	}

	parsedURL, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || parsedURL == nil || parsedURL.Hostname() == "" {
		return nil, "", nil, ErrInvalidRequest
	}
	switch parsedURL.Scheme {
	case "http", "https":
	default:
		return nil, "", nil, ErrInvalidRequest
	}

	if (method == "GET" || method == "HEAD") && len(req.Body) > 0 {
		return nil, "", nil, ErrInvalidRequest
	}

	return parsedURL, method, append([]byte(nil), req.Body...), nil
}

func toHostSet(hosts []string) map[string]struct{} {
	items := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		normalized := normalizeHost(host)
		if normalized == "" {
			continue
		}
		items[normalized] = struct{}{}
	}
	return items
}

func normalizeHost(host string) string {
	normalized := strings.ToLower(strings.TrimSpace(host))
	normalized = strings.TrimSuffix(normalized, ".")
	return normalized
}
