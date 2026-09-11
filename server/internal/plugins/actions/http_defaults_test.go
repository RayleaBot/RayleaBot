package actions

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestHTTPFallbackMatchesFreshConfigAndPreservesZeroRetries(t *testing.T) {
	defaults, _, err := config.Load(filepath.Join(t.TempDir(), "missing.yaml"), config.ConfigUserSchemaID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{HTTP: config.HTTPConfig{MaxRetries: -1}}
	if retries := currentHTTPMaxRetries(cfg); retries != defaults.HTTP.MaxRetries {
		t.Fatalf("fallback retries = %d, fresh config = %d", retries, defaults.HTTP.MaxRetries)
	}
	if timeout := currentHTTPTimeout(cfg); timeout != time.Duration(defaults.HTTP.TimeoutSeconds)*time.Second {
		t.Fatalf("fallback timeout = %s", timeout)
	}
	cfg.HTTP.MaxRetries = 0
	if retries := currentHTTPMaxRetries(cfg); retries != 0 {
		t.Fatalf("explicit zero retries = %d", retries)
	}
}
