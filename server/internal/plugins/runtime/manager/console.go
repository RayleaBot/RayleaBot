package manager

import (
	"io"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
	runtimespec "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/spec"
)

const (
	defaultConsoleChunkBytes     = 4096
	defaultStderrRateLimitPerSec = 262144
	stderrTruncatedSystemMessage = "[系统] stderr 输出超过速率限制，后续内容已截断"
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
					"插件"+pluginIDLabel(pluginID)+"运行时 stderr 输出超过速率限制，已截断",
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
			"插件"+pluginIDLabel(pluginID)+"运行时 stderr 读取失败",
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

func runtimePluginLabel(spec runtimespec.Spec) string {
	pluginID := strings.TrimSpace(spec.PluginID)
	name := strings.TrimSpace(spec.PluginName)
	switch {
	case name != "" && pluginID != "" && name != pluginID:
		return name + "（" + pluginID + "）"
	case name != "":
		return name
	default:
		return pluginID
	}
}

func pluginIDLabel(pluginID string) string {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return "未知插件"
	}
	return pluginID
}
