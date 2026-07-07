package redact

import (
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

const placeholder = "[REDACTED]"

type Redactor struct {
	mu     sync.RWMutex
	values []string
}

func New(values ...string) *Redactor {
	r := &Redactor{}
	r.Add(values...)
	return r
}

func (r *Redactor) Add(values ...string) {

	normalized := normalizeValues(values)
	if len(normalized) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	merged := append(append([]string(nil), r.values...), normalized...)
	r.values = normalizeValues(merged)
}

func (r *Redactor) Redact(text string) string {
	if text == "" {
		return text
	}

	r.mu.RLock()
	values := append([]string(nil), r.values...)
	r.mu.RUnlock()

	for _, value := range values {
		text = strings.ReplaceAll(text, value, placeholder)
	}

	return text
}

func normalizeValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) < 4 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	slices.SortFunc(result, func(left, right string) int {
		if len(left) == len(right) {
			return strings.Compare(left, right)
		}
		if len(left) > len(right) {
			return -1
		}
		return 1
	})

	return result
}

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
