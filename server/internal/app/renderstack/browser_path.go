package renderstack

import (
	"context"
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
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
				"托管 Chromium 暂不可用，图片渲染等待运行环境准备",
				"component", "render",
				"code", "platform.resource_missing",
				"err", logpath.Error(repoRoot, err, repoRoot),
			)
		}
		return ""
	}

	if logger != nil {
		browserDisplayPath := logpath.Display(repoRoot, managedBrowserPath)
		logger.Info(
			"托管 Chromium 已就绪，浏览器路径："+browserDisplayPath,
			"component", "render",
			"browser_path", browserDisplayPath,
		)
	}
	return managedBrowserPath
}
