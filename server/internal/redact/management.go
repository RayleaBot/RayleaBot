package redact

import (
	"os"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func NewManagementRedactor(cfg config.Config) *Redactor {
	values := []string{
		cfg.OneBot.ReverseWS.AccessToken,
		cfg.OneBot.ForwardWS.AccessToken,
		cfg.OneBot.HTTPAPI.AccessToken,
		cfg.OneBot.Webhook.AccessToken,
	}
	values = append(values, sensitiveEnvironmentValues(os.Environ())...)
	return New(values...)
}

func sensitiveEnvironmentValues(env []string) []string {
	result := make([]string, 0, len(env))
	for _, binding := range env {
		name, value, ok := strings.Cut(binding, "=")
		if !ok || !isSensitiveEnvName(name) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func isSensitiveEnvName(name string) bool {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "RAYLEABOT_SECRET_") {
		return true
	}

	keywords := []string{
		"SECRET",
		"TOKEN",
		"PASSWORD",
		"PASSWD",
		"API_KEY",
		"ACCESS_KEY",
		"PRIVATE_KEY",
	}
	for _, keyword := range keywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}
	if strings.HasSuffix(name, "_KEY") || strings.Contains(name, "_KEY_") {
		return true
	}

	return false
}
