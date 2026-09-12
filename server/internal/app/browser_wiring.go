package app

import (
	"log/slog"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/browser"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type browserRenderer interface {
	BrowserLaunchConfig() (string, []string)
}

type browserWiringDeps struct {
	Config   config.Config
	Renderer browserRenderer
	Logger   *slog.Logger
	RepoRoot string
}

type browserWiringState struct {
	Manager *browser.Manager
}

func buildBrowserManager(deps browserWiringDeps) browserWiringState {
	managedPath := deps.Config.Render.BrowserPath
	browserArgs := deps.Config.Render.BrowserArgs
	if deps.Renderer != nil {
		managedPath, browserArgs = deps.Renderer.BrowserLaunchConfig()
	}
	return browserWiringState{Manager: browser.NewManager(browser.Options{
		ConfiguredBrowserPath: deps.Config.Render.BrowserPath,
		ManagedBrowserPath:    managedPath,
		BrowserArgs:           browserArgs,
		ProfileRoot:           pluginBrowserProfileRoot(deps.RepoRoot),
		Logger:                deps.Logger,
	})}
}

// pluginBrowserProfileRoot 返回插件浏览器会话的持久化 profile 根目录。
// RepoRoot 为空时返回空串，会话退化为临时 profile（不持久化）。
func pluginBrowserProfileRoot(repoRoot string) string {
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, "data", "plugin-browser")
}
