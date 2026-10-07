package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func DefaultUpdateConfig() UpdateConfig {
	payload, _ := json.Marshal(defaultDocumentTemplate["update"])
	var settings UpdateConfig
	_ = json.Unmarshal(payload, &settings)
	return settings
}

// NormalizeUpdatePrefix accepts the forms users copy from GitHub accelerators.
// The stored value is always a base prefix, never a repository-specific URL.
func NormalizeUpdatePrefix(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if strings.HasSuffix(value, "{url}") {
		value = strings.TrimSuffix(value, "{url}")
	}
	for _, host := range []string{"github.com", "api.github.com", "raw.githubusercontent.com"} {
		if strings.HasSuffix(value, "/https://"+host) {
			value = strings.TrimSuffix(value, "/https://"+host)
			break
		}
		if i := strings.Index(value, "/https://"+host+"/"); i >= 0 {
			value = value[:i]
			break
		}
	}
	return normalizeUpdateBase(value)
}

func normalizeUpdateBase(value string) (string, error) {
	u, err := url.Parse(strings.TrimRight(value, "/"))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(value, "{}\\\r\n\t ") {
		return "", fmt.Errorf("update source must be an HTTPS base URL without credentials, query or fragment")
	}
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

func normalizeUpdateDocument(document map[string]any) error {
	section, ok := document["update"].(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range []string{"proxies", "mirrors"} {
		values, ok := section[key].([]any)
		if !ok {
			continue
		}
		seen := map[string]bool{}
		out := make([]any, 0, len(values))
		for _, raw := range values {
			value, ok := raw.(string)
			if !ok {
				return fmt.Errorf("update.%s must contain URLs", key)
			}
			var normalized string
			var err error
			if key == "proxies" {
				normalized, err = NormalizeUpdatePrefix(value)
			} else {
				normalized, err = normalizeUpdateBase(strings.TrimSpace(value))
			}
			if err != nil {
				return fmt.Errorf("update.%s: %w", key, err)
			}
			if !seen[normalized] {
				out = append(out, normalized)
				seen[normalized] = true
			}
		}
		section[key] = out
	}
	return nil
}

func configUpdateDocument(cfg Config) any {
	settings := cfg.Update
	if settings.Channel == "" {
		settings = DefaultUpdateConfig()
	}
	if settings.Proxies == nil {
		settings.Proxies = []string{}
	}
	if settings.Mirrors == nil {
		settings.Mirrors = []string{}
	}
	payload, _ := json.Marshal(settings)
	var document map[string]any
	_ = json.Unmarshal(payload, &document)
	return document
}
