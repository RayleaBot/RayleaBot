package shell

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func isAuthFailure(response *http.Response) bool {
	if response == nil {
		return false
	}

	return response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden
}
func summarizeError(err error) string {
	if err == nil {
		return ""
	}

	return strings.Join(strings.Fields(err.Error()), " ")
}
func sanitizeWSURL(raw string) string {
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}
func dialURL(raw, accessToken string, includeTokenQuery bool) string {
	if raw == "" || accessToken == "" || !includeTokenQuery {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	query := parsed.Query()
	query.Set("access_token", accessToken)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
func (s *Shell) waitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.deps.connectTimeout <= 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, s.deps.connectTimeout)
}
func (s *Shell) provisionalReadTimeout(snapshot Snapshot) time.Duration {
	if snapshot.HeartbeatInterval > 0 {
		return snapshot.HeartbeatInterval * 3
	}
	if snapshot.State == StateConnected {
		if s.deps.connectTimeout > defaultConnectedReadTimeout {
			return s.deps.connectTimeout
		}
		return defaultConnectedReadTimeout
	}
	if s.deps.connectTimeout > 0 {
		return s.deps.connectTimeout
	}

	return time.Second
}
func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func waitForClosed(ctx context.Context, ch <-chan struct{}) error {
	if ch == nil {
		return nil
	}

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}
