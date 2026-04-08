package logging

import "strings"

const ProtocolOneBot11 = "onebot11"

func NormalizeSummary(summary Summary) Summary {
	summary.LogID = strings.TrimSpace(summary.LogID)
	summary.Level = strings.ToLower(strings.TrimSpace(summary.Level))
	summary.Source = strings.TrimSpace(summary.Source)
	summary.Message = strings.TrimSpace(summary.Message)
	summary.PluginID = strings.TrimSpace(summary.PluginID)
	summary.RequestID = strings.TrimSpace(summary.RequestID)
	summary.Protocol = strings.TrimSpace(summary.Protocol)
	summary.Details = sanitizeDetailsMap(cloneDetailsMap(summary.Details))

	if summary.LogID == "" {
		summary.LogID = generateLogID()
	}

	if summary.Source == "" {
		summary.Source = "server"
	}
	if summary.Protocol == "" {
		summary.Protocol = protocolFromSource(summary.Source)
	}

	return summary
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
