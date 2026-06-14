package manager

import (
	"io"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
)

const (
	defaultConsoleChunkBytes     = 4096
	defaultStderrRateLimitPerSec = 262144
	stderrTruncatedSystemMessage = "[System] stderr rate limit exceeded, output truncated"
)

type stderrLimiter struct {
	now         func() time.Time
	limit       int
	windowStart time.Time
	used        int
	truncated   bool
}

func newStderrLimiter(limit int, now func() time.Time) *stderrLimiter {
	if limit <= 0 {
		limit = defaultStderrRateLimitPerSec
	}
	if now == nil {
		now = time.Now
	}

	return &stderrLimiter{
		now:   now,
		limit: limit,
	}
}

func (l *stderrLimiter) allow(chunk []byte) ([]byte, bool) {
	if len(chunk) == 0 {
		return nil, false
	}

	current := l.now().UTC()
	if l.windowStart.IsZero() || current.Sub(l.windowStart) >= time.Second {
		l.windowStart = current
		l.used = 0
		l.truncated = false
	}

	remaining := l.limit - l.used
	if remaining <= 0 {
		if l.truncated {
			return nil, false
		}
		l.truncated = true
		return nil, true
	}

	if len(chunk) <= remaining {
		l.used += len(chunk)
		return append([]byte(nil), chunk...), false
	}

	allowed := append([]byte(nil), chunk[:remaining]...)
	l.used += remaining
	if l.truncated {
		return allowed, false
	}
	l.truncated = true
	return allowed, true
}

func (m *Manager) captureStderr(pluginID string, reader io.ReadCloser) {
	if reader == nil {
		return
	}
	defer reader.Close()

	limiter := newStderrLimiter(m.opts.StderrRateLimitBytesPerSec, m.deps.now)
	buffer := make([]byte, defaultConsoleChunkBytes)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			allowed, truncated := limiter.allow(buffer[:n])
			if len(allowed) > 0 {
				m.appendConsoleEntry(console.Entry{
					PluginID:  pluginID,
					Stream:    "stderr",
					Text:      string(allowed),
					Timestamp: m.deps.now().UTC(),
				})
			}
			if truncated {
				m.logger.Warn(
					"plugin runtime stderr truncated",
					"component", "runtime",
					"plugin_id", pluginID,
				)
				m.appendConsoleEntry(console.Entry{
					PluginID:  pluginID,
					Stream:    "system",
					Text:      stderrTruncatedSystemMessage,
					Timestamp: m.deps.now().UTC(),
				})
			}
		}

		if err == nil {
			continue
		}
		if err == io.EOF {
			return
		}
		m.logger.Warn(
			"plugin runtime stderr stream failed",
			"component", "runtime",
			"plugin_id", pluginID,
			"err", err.Error(),
		)
		return
	}
}

func (m *Manager) appendConsoleEntry(entry console.Entry) {
	if m == nil || m.opts.Console == nil || entry.Text == "" {
		return
	}

	entry.Text = m.opts.RedactText(entry.Text)
	if entry.Text == "" {
		return
	}

	m.opts.Console.Append(entry)
}
