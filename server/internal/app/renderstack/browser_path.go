package renderstack

import (
	"context"
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

var resolveManagedBrowserPath = func(ctx context.Context, repoRoot string) (string, error) {
	return deps.NewRuntime(repoRoot).ResolveEntrypoint(ctx, "chromium", "browser")
}

func prepareBrowserPath(ctx context.Context, logger *slog.Logger, repoRoot string, configuredPath string) string {
	browserPath := strings.TrimSpace(configuredPath)
	if browserPath != "" {
		return browserPath
	}

	managedBrowserPath, err := resolveManagedBrowserPath(ctx, repoRoot)
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
