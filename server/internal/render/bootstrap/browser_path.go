package bootstrap

import (
	"context"
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

var ResolveManagedBrowserPath = func(ctx context.Context, repoRoot string) (string, error) {
	return deps.NewManager(repoRoot).ResolveEntrypoint(ctx, "chromium", "browser")
}

func PrepareBrowserPath(ctx context.Context, logger *slog.Logger, repoRoot string, configuredPath string) string {
	browserPath := strings.TrimSpace(configuredPath)
	if browserPath != "" {
		return browserPath
	}

	managedBrowserPath, err := ResolveManagedBrowserPath(ctx, repoRoot)
	if err != nil {
		if logger != nil {
			logger.Warn(
				"managed chromium bootstrap pending",
				"component", "render",
				"code", "platform.resource_missing",
				"err", err.Error(),
			)
		}
		return ""
	}

	if logger != nil {
		logger.Info(
			"managed chromium bootstrap ready",
			"component", "render",
			"browser_path", managedBrowserPath,
		)
	}
	return managedBrowserPath
}
