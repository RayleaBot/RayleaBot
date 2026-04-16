package logging

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

const ProtocolOneBot11 = "onebot11"

func NormalizeSummary(summary Summary) Summary {
	summary.BootID = strings.TrimSpace(summary.BootID)
	summary.LogID = strings.TrimSpace(summary.LogID)
	summary.Timestamp = normalizeSummaryTimestamp(summary.Timestamp)
	summary.Level = strings.ToLower(strings.TrimSpace(summary.Level))
	summary.Source = strings.TrimSpace(summary.Source)
	summary.Message = strings.TrimSpace(summary.Message)
	summary.PluginID = strings.TrimSpace(summary.PluginID)
	summary.RequestID = strings.TrimSpace(summary.RequestID)
	summary.Protocol = strings.TrimSpace(summary.Protocol)

	if summary.LogID == "" {
		summary.LogID = generateLogID()
	}

	if summary.Source == "" {
		summary.Source = "server"
	}
	if summary.Protocol == "" {
		summary.Protocol = protocolFromSource(summary.Source)
	}
	if summary.Protocol == ProtocolOneBot11 {
		summary.Message = strings.TrimSpace(textsafe.SanitizeString(summary.Message))
	}
	summary.Details = normalizeProtocolDetails(summary.Protocol, summary.Details)

	return summary
}

func normalizeSummaryTimestamp(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return trimmed
	}

	return parsed.UTC().Format(time.RFC3339Nano)
}

func IsSupportedProtocol(protocol string) bool {
	switch strings.TrimSpace(protocol) {
	case ProtocolOneBot11:
		return true
	default:
		return false
	}
}

func protocolFromSource(source string) string {
	switch strings.TrimSpace(source) {
	case "adapter", "adapter.onebot11", "bridge":
		return ProtocolOneBot11
	default:
		return ""
	}
}

func protocolSources(protocol string) []string {
	switch strings.TrimSpace(protocol) {
	case ProtocolOneBot11:
		return []string{"adapter", "adapter.onebot11", "bridge"}
	default:
		return nil
	}
}
