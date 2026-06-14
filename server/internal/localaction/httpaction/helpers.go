package httpaction

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
)

const (
	defaultTimeoutSeconds = 10
	defaultMaxRetries     = 2
)

func currentTimeout(cfg config.Config) time.Duration {
	seconds := cfg.HTTP.TimeoutSeconds
	if seconds <= 0 {
		seconds = defaultTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func currentMaxRetries(cfg config.Config) int {
	if cfg.HTTP.MaxRetries < 0 {
		return defaultMaxRetries
	}
	if cfg.HTTP.MaxRetries == 0 {
		return 0
	}
	return cfg.HTTP.MaxRetries
}

func currentActionTimeout(action runtimeaction.Action) time.Duration {
	if action.HTTPTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(action.HTTPTimeoutSeconds) * time.Second
}

func CloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}
