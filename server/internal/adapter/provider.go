package adapter

import "strings"

const (
	ProviderUnknown     = "unknown"
	ProviderStandard    = "standard"
	ProviderNapCat      = "napcat"
	ProviderLuckyLillia = "luckylillia"
)

func DetectProvider(appName string) string {
	normalized := strings.ToLower(strings.TrimSpace(appName))
	switch {
	case normalized == "":
		return ProviderUnknown
	case strings.Contains(normalized, "napcat"):
		return ProviderNapCat
	case strings.Contains(normalized, "llonebot"), strings.Contains(normalized, "luckylillia"):
		return ProviderLuckyLillia
	default:
		return ProviderStandard
	}
}
