package desktop

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

func (c *Coordinator) SaveSettings(settings LauncherSettings) error {
	normalized, err := normalizeSettings(settings, "")
	if err != nil {
		return err
	}
	unblockStartups, cancelledStartup := c.startups.blockWithState(false)
	defer unblockStartups()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	if cancelledStartup && c.Snapshot().Launcher.ProcessLifecycle == "starting" && c.process.IsRunning() {
		if err := c.process.ForceKill(); err != nil {
			return fmt.Errorf("无法停止正在使用旧设置启动的服务：%w", err)
		}
	}
	if err := c.settingsStore.Save(normalized); err != nil {
		return err
	}
	c.mu.Lock()
	c.settings = normalized
	c.mu.Unlock()
	if !c.process.IsRunning() {
		c.process.SetWorkdir(ResolveLauncherSettings(normalized).Workdir)
	}
	return c.refreshCurrent()
}

func (c *Coordinator) PreviewResolvedSettings(settings LauncherSettings) (LauncherResolvedSettings, error) {
	normalized, err := normalizeSettings(settings, "")
	if err != nil {
		return LauncherResolvedSettings{}, err
	}
	return ResolveLauncherSettings(normalized), nil
}

func (c *Coordinator) OpenWebUI(targetPath string) error {
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	normalized, err := sanitizeWebTargetPath(targetPath)
	if err != nil {
		return err
	}
	baseURL := operation.endpoint.BaseURL
	if override := strings.TrimSpace(os.Getenv("RAYLEA_WEB_UI_BASE_URL")); override != "" {
		if candidate, parseErr := url.Parse(override); parseErr == nil && (candidate.Scheme == "http" || candidate.Scheme == "https") && candidate.User == nil {
			baseURL = candidate.String()
		}
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	if normalized != "" {
		base, err = base.Parse(normalized)
		if err != nil {
			return err
		}
	} else if snapshot := c.Snapshot(); snapshot.Server.Readiness != nil && readinessStatus(snapshot.Server.Readiness) == "setup_required" {
		if token := c.process.SetupToken(); token != "" {
			base.Path = "/setup"
			base.Fragment = "setup_token=" + token
		}
	}
	return c.host.OpenURL(base.String())
}

func (c *Coordinator) OpenReleasePage() error {
	snapshot := c.Snapshot()
	page := snapshot.Launcher.ReleaseCheck.ReleasePageURL
	if page == "" {
		snapshot.Launcher.StatusHint = "没有可打开的版本页面。"
		c.publish(snapshot)
		return nil
	}
	return c.host.OpenURL(page)
}

func (c *Coordinator) OpenRepositoryPage() error {
	return c.host.OpenURL(repositoryURL)
}

func (c *Coordinator) OpenLogsDirectory() error {
	directory := c.process.LogDirectory()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	return c.host.OpenDirectory(directory)
}

func sanitizeWebTargetPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\\\r\n\x00") {
		return "", errors.New("管理界面目标必须是安全的站内绝对路径")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil {
		return "", errors.New("管理界面目标路径无效")
	}
	for _, segment := range strings.Split(parsed.EscapedPath(), "/") {
		if segment == "." || segment == ".." || strings.EqualFold(segment, "%2e") || strings.EqualFold(segment, "%2e%2e") {
			return "", errors.New("管理界面目标路径不能包含目录跳转")
		}
	}
	return parsed.String(), nil
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
