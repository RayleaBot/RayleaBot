package action

import (
	"encoding/json"
	"strings"
)

func parseHTTPRequestAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionHTTPRequestFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed http.request data", err)
	}

	method := strings.ToUpper(strings.TrimSpace(frame.Method))
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported http.request method", nil)
	}

	targetURL := strings.TrimSpace(frame.URL)
	if targetURL == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required http.request fields", nil)
	}

	body, err := decodeExclusiveTextOrBase64(frame.BodyText, frame.BodyBase64, false)
	if err != nil {
		return nil, err
	}
	if (method == "GET" || method == "HEAD") && len(body) > 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported http.request body for method", nil)
	}

	timeoutSeconds := 0
	if frame.TimeoutSeconds != nil {
		timeoutSeconds = *frame.TimeoutSeconds
		if timeoutSeconds <= 0 {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid http.request timeout_seconds", nil)
		}
	}

	return &Action{
		Kind:               "http.request",
		HTTPMethod:         method,
		HTTPURL:            targetURL,
		HTTPHeaders:        cloneHTTPActionHeaders(frame.Headers),
		HTTPTimeoutSeconds: timeoutSeconds,
		HTTPBody:           body,
	}, nil
}
