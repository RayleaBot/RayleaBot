package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type depsManifest struct {
	ManifestVersion int            `json:"manifest_version"`
	Resources       []depsResource `json:"resources"`
}

type depsResource struct {
	ID            string               `json:"id"`
	Kind          string               `json:"kind"`
	Version       string               `json:"version"`
	Platform      string               `json:"platform"`
	Sources       []depsResourceSource `json:"sources"`
	SHA256        string               `json:"sha256"`
	ArchiveFormat string               `json:"archive_format"`
	Entrypoints   map[string][]string  `json:"entrypoints"`
}

type depsResourceSource struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

func InspectEnvironment(settings LauncherResolvedSettings) EnvironmentInspection {
	return inspectEnvironment(settings, true, false)
}

func inspectEnvironmentPassive(settings LauncherResolvedSettings, previouslyWritable bool) EnvironmentInspection {
	return inspectEnvironment(settings, false, previouslyWritable)
}

func inspectEnvironment(settings LauncherResolvedSettings, probeWorkdir bool, previouslyWritable bool) EnvironmentInspection {
	checks := []EnvironmentCheckResult{
		checkPath("launcher.installation_root", "launcher.installation_root_missing", "安装目录", pathExists(settings.InstallationRoot), "安装目录可用。", "安装目录不存在或无法访问。", "请选择有效的 RayleaBot 安装目录。", true),
		checkPath("launcher.settings", "launcher.settings_invalid", "启动器设置", settings.ServerExecutablePath != "" && settings.ConfigPath != "" && settings.Workdir != "", "启动器设置已解析。", "服务端、配置或工作目录未能解析。", "请检查安装目录和高级路径覆盖。", true),
		checkPath("server.executable", "server.executable_missing", "服务端可执行文件", regularFile(settings.ServerExecutablePath), "已找到 raylea-server。", fmt.Sprintf("未找到服务端可执行文件：%s", settings.ServerExecutablePath), "请选择有效的 raylea-server 可执行文件。", true),
	}

	userConfigExists := regularFile(settings.ConfigPath)
	switch {
	case userConfigExists:
		checks = append(checks, okCheck("config.file", "用户配置", "config/user.yaml 可用。"))
	case !pathExists(settings.ConfigPath) && regularFile(settings.ServerExecutablePath):
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "config.bootstrap_available", Title: "用户配置", Severity: "warning",
			Summary: "首次启动时将自动生成用户配置。", Detail: fmt.Sprintf("尚未找到 %s", settings.ConfigPath),
			Remediation: "启动服务时会按内嵌默认值生成 config/user.yaml。",
		})
	default:
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "config.missing", Title: "用户配置", Severity: "error",
			Summary: "无法读取或初始化用户配置。", Detail: "配置路径必须是文件，初始化需要可用的服务端程序。",
			Remediation: "请选择有效的服务端程序与用户配置文件路径。",
		})
	}

	writable := workdirWritable(settings.Workdir, probeWorkdir, previouslyWritable)
	checks = append(checks, checkPath("workdir.ready", "workdir.unwritable", "工作目录", writable, "工作目录可写。", fmt.Sprintf("无法写入：%s", settings.Workdir), "请选择可写的工作目录。", true))
	checks = append(checks, inspectRuntimeManifest(settings.InstallationRoot)...)

	inspection := EnvironmentInspection{
		Checks:          checks,
		PreflightChecks: append([]EnvironmentCheckResult(nil), checks...),
		AdvisoryChecks:  []EnvironmentCheckResult{},
	}
	for _, check := range checks {
		inspection.HasBlockingIssues = inspection.HasBlockingIssues || check.Severity == "error"
		inspection.CanBootstrapUserConfig = inspection.CanBootstrapUserConfig || check.Code == "config.bootstrap_available"
	}
	return inspection
}

func inspectRuntimeManifest(root string) []EnvironmentCheckResult {
	manifestPath := filepath.Join(root, ".deps", "manifest.json")
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		checks := []EnvironmentCheckResult{{
			Scope: "preflight", Code: "deps.manifest_missing", Title: "运行环境清单", Severity: "warning",
			Summary: ".deps/manifest.json 未找到。", Detail: fmt.Sprintf("检查路径：%s", manifestPath), Remediation: "请恢复运行环境清单。",
		}}
		if browser := findSystemChromium(); browser != "" {
			checks = append(checks, chromiumReadyCheck(browser))
		}
		return checks
	}

	var manifest depsManifest
	if json.Unmarshal(payload, &manifest) != nil || manifest.ManifestVersion != 5 {
		return []EnvironmentCheckResult{{
			Scope: "preflight", Code: "deps.manifest_invalid", Title: "运行环境清单", Severity: "warning",
			Summary: ".deps/manifest.json 内容无效。", Detail: fmt.Sprintf("检查路径：%s", manifestPath), Remediation: "请恢复 manifest_version 5 的运行环境清单。",
		}}
	}

	platform := manifestPlatform()
	var chromium *depsResource
	var ffmpeg *depsResource
	foundPlatform := false
	for index := range manifest.Resources {
		resource := &manifest.Resources[index]
		if resource.Platform == platform {
			foundPlatform = true
			if resource.Kind == "chromium" {
				chromium = resource
			} else if resource.Kind == "ffmpeg" {
				ffmpeg = resource
			}
		}
	}
	checks := []EnvironmentCheckResult{}
	if foundPlatform {
		checks = append(checks, okCheck("deps.manifest", "运行环境清单", fmt.Sprintf("已包含当前平台资源：%s", platform)))
	} else {
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "deps.manifest_platform_missing", Title: "运行环境清单", Severity: "warning",
			Summary: "运行环境清单缺少当前平台资源。", Detail: fmt.Sprintf("平台：%s", platform), Remediation: "请恢复当前平台的运行环境资源。",
		})
	}
	if chromium == nil {
		if browser := findSystemChromium(); browser != "" {
			checks = append(checks, chromiumReadyCheck(browser))
		} else {
			checks = append(checks, EnvironmentCheckResult{
				Scope: "preflight", Code: "chromium.resource_missing", Title: "图片渲染 Chromium", Severity: "warning",
				Summary: "清单中未配置 Chromium。", Remediation: "恢复 Chromium 运行时资源，或安装可用的 Chrome、Edge 或 Chromium。",
			})
		}
	} else if !resourceMetadataComplete(*chromium) {
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "chromium.metadata_incomplete", Title: "图片渲染 Chromium", Severity: "warning",
			Summary: "Chromium 资源元数据不完整。", Remediation: "请恢复来源、校验值、压缩格式和 browser 入口。",
		})
	} else {
		checks = append(checks, inspectChromiumState(root, *chromium, findSystemChromium))
	}
	if ffmpeg == nil {
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "ffmpeg.resource_missing", Title: "媒体工具 FFmpeg", Severity: "warning",
			Summary: "清单中未配置 FFmpeg。", Remediation: "请恢复当前平台的 FFmpeg / FFprobe 运行时资源。",
		})
	} else if !resourceMetadataComplete(*ffmpeg) {
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "ffmpeg.metadata_incomplete", Title: "媒体工具 FFmpeg", Severity: "warning",
			Summary: "FFmpeg 资源元数据不完整。", Remediation: "请恢复来源、校验值、压缩格式和 ffmpeg / ffprobe 入口。",
		})
	} else {
		checks = append(checks, inspectFFmpegState(root, *ffmpeg))
	}
	return checks
}

func inspectChromiumState(root string, chromium depsResource, findBrowser func() string) EnvironmentCheckResult {
	storeRoot := filepath.Join(root, ".deps", "store", chromium.ID, chromium.Version)
	for _, relative := range chromium.Entrypoints["browser"] {
		if validRelativeEntrypoint(relative) && regularFile(filepath.Join(storeRoot, filepath.FromSlash(relative))) {
			return chromiumReadyCheck(filepath.Join(storeRoot, filepath.FromSlash(relative)))
		}
	}
	if browser := findBrowser(); browser != "" {
		return chromiumReadyCheck(browser)
	}
	code := "chromium.not_ready"
	summary := "Chromium 尚未准备。"
	detail := fmt.Sprintf("运行时目录：%s", storeRoot)
	archivePath := filepath.Join(root, "cache", "downloads", "runtime", chromium.ID+"-"+chromium.Version+runtimeArchiveSuffix(chromium.ArchiveFormat))
	tempRoots := findRuntimeTempRoots(filepath.Dir(storeRoot), chromium.ID, chromium.Version)
	switch {
	case len(tempRoots) > 0 && !pathExists(storeRoot):
		code = "chromium.extract_incomplete"
		summary = "Chromium 上次解压未完成。"
		detail = fmt.Sprintf("下载位置：%s。解压位置：%s。临时目录：%s", archivePath, storeRoot, strings.Join(tempRoots, "、"))
	case pathExists(storeRoot):
		code = "chromium.entrypoint_missing"
		summary = "Chromium 已解压，但入口文件缺失。"
	case regularFile(archivePath):
		summary = "Chromium 已下载，尚未解压。"
		detail = fmt.Sprintf("下载位置：%s。解压位置：%s", archivePath, storeRoot)
	}
	return EnvironmentCheckResult{
		Scope: "preflight", Code: code, Title: "图片渲染 Chromium", Severity: "warning",
		Summary: summary, Detail: detail, Remediation: "启动服务后由运行环境任务重新准备 Chromium。",
	}
}

func inspectFFmpegState(root string, ffmpeg depsResource) EnvironmentCheckResult {
	storeRoot := filepath.Join(root, ".deps", "store", ffmpeg.ID, ffmpeg.Version)
	ready := true
	paths := make([]string, 0, 2)
	for _, key := range []string{"ffmpeg", "ffprobe"} {
		found := ""
		for _, relative := range ffmpeg.Entrypoints[key] {
			candidate := filepath.Join(storeRoot, filepath.FromSlash(relative))
			if validRelativeEntrypoint(relative) && regularFile(candidate) {
				found = candidate
				break
			}
		}
		ready = ready && found != ""
		if found != "" {
			paths = append(paths, found)
		}
	}
	if ready {
		return EnvironmentCheckResult{Scope: "preflight", Code: "ffmpeg.ready", Title: "媒体工具 FFmpeg", Severity: "ok", Summary: "已找到 FFmpeg 与 FFprobe。", Detail: strings.Join(paths, "；")}
	}
	code := "ffmpeg.not_ready"
	summary := "FFmpeg 尚未准备。"
	detail := fmt.Sprintf("运行时目录：%s", storeRoot)
	archivePath := filepath.Join(root, "cache", "downloads", "runtime", ffmpeg.ID+"-"+ffmpeg.Version+runtimeArchiveSuffix(ffmpeg.ArchiveFormat))
	tempRoots := findRuntimeTempRoots(filepath.Dir(storeRoot), ffmpeg.ID, ffmpeg.Version)
	switch {
	case len(tempRoots) > 0 && !pathExists(storeRoot):
		code = "ffmpeg.extract_incomplete"
		summary = "FFmpeg 上次解压未完成。"
		detail = fmt.Sprintf("下载位置：%s。解压位置：%s。临时目录：%s", archivePath, storeRoot, strings.Join(tempRoots, "、"))
	case pathExists(storeRoot):
		code = "ffmpeg.entrypoint_missing"
		summary = "FFmpeg 已解压，但 ffmpeg 或 ffprobe 入口缺失。"
	case regularFile(archivePath):
		summary = "FFmpeg 已下载，尚未解压。"
		detail = fmt.Sprintf("下载位置：%s。解压位置：%s", archivePath, storeRoot)
	}
	return EnvironmentCheckResult{
		Scope: "preflight", Code: code, Title: "媒体工具 FFmpeg", Severity: "warning",
		Summary: summary, Detail: detail, Remediation: "启动服务后由运行环境任务重新准备 FFmpeg。",
	}
}

func runtimeArchiveSuffix(format string) string {
	switch format {
	case "tar.gz":
		return ".tar.gz"
	case "tar.xz":
		return ".tar.xz"
	default:
		return ".zip"
	}
}

func findRuntimeTempRoots(parent, id, version string) []string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	prefix := "." + id + "-" + version + "-"
	result := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && strings.HasPrefix(entry.Name(), prefix) {
			result = append(result, filepath.Join(parent, entry.Name()))
		}
	}
	return result
}

func resourceMetadataComplete(resource depsResource) bool {
	if resource.ID == "" || resource.Version == "" || len(resource.SHA256) != 64 {
		return false
	}
	switch resource.ArchiveFormat {
	case "zip", "tar.gz", "tar.xz":
	default:
		return false
	}
	entrypointKeys := []string{"browser"}
	if resource.Kind == "ffmpeg" {
		entrypointKeys = []string{"ffmpeg", "ffprobe"}
	} else if resource.Kind != "chromium" {
		return false
	}
	if len(resource.Sources) == 0 {
		return false
	}
	for _, source := range resource.Sources {
		if !strings.HasPrefix(source.URL, "https://") || (source.Kind != "upstream" && source.Kind != "mirror") {
			return false
		}
	}
	for _, key := range entrypointKeys {
		valid := false
		for _, candidate := range resource.Entrypoints[key] {
			if validRelativeEntrypoint(candidate) {
				valid = true
				break
			}
		}
		if !valid {
			return false
		}
	}
	return true
}

func validRelativeEntrypoint(value string) bool {
	value = strings.TrimSpace(filepath.ToSlash(value))
	hasDrivePrefix := len(value) >= 2 && value[1] == ':' && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z'))
	return value != "" && !hasDrivePrefix && !filepath.IsAbs(value) && !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "../") && !strings.Contains(value, "/../")
}

func manifestPlatform() string {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	return platform + "-" + arch
}

func findSystemChromium() string {
	home, _ := os.UserHomeDir()
	candidates := systemChromiumCandidates(runtime.GOOS, os.Getenv, home)
	for _, candidate := range candidates {
		if regularFile(candidate) {
			return candidate
		}
	}
	for _, name := range []string{"msedge.exe", "chrome.exe", "chromium.exe", "google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "microsoft-edge-stable"} {
		if candidate, err := lookPath(name); err == nil {
			return candidate
		}
	}
	return ""
}

func systemChromiumCandidates(goos string, getenv func(string) string, userHome string) []string {
	candidates := []string{}
	add := func(parts ...string) {
		candidate := filepath.Join(parts...)
		if candidate != "" {
			candidates = append(candidates, candidate)
		}
	}
	switch goos {
	case "windows":
		for _, root := range []string{getenv("ProgramFiles"), getenv("ProgramFiles(x86)"), getenv("LocalAppData")} {
			if root == "" {
				continue
			}
			add(root, "Microsoft", "Edge", "Application", "msedge.exe")
			add(root, "Google", "Chrome", "Application", "chrome.exe")
			add(root, "Chromium", "Application", "chromium.exe")
		}
	case "darwin":
		add("/Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome")
		add("/Applications", "Chromium.app", "Contents", "MacOS", "Chromium")
		add("/Applications", "Microsoft Edge.app", "Contents", "MacOS", "Microsoft Edge")
		if strings.TrimSpace(userHome) != "" {
			add(userHome, "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome")
			add(userHome, "Applications", "Chromium.app", "Contents", "MacOS", "Chromium")
			add(userHome, "Applications", "Microsoft Edge.app", "Contents", "MacOS", "Microsoft Edge")
		}
	default:
		candidates = append(candidates, "/usr/bin/google-chrome", "/usr/bin/google-chrome-stable", "/usr/bin/chromium", "/usr/bin/chromium-browser", "/snap/bin/chromium", "/opt/microsoft/msedge/msedge")
	}
	return candidates
}

func checkPath(okCode, errorCode, title string, ready bool, okSummary, errorDetail, remediation string, blocking bool) EnvironmentCheckResult {
	if ready {
		return okCheck(okCode, title, okSummary)
	}
	severity := "warning"
	if blocking {
		severity = "error"
	}
	return EnvironmentCheckResult{Scope: "preflight", Code: errorCode, Title: title, Severity: severity, Summary: errorDetail, Detail: errorDetail, Remediation: remediation}
}

func okCheck(code, title, summary string) EnvironmentCheckResult {
	return EnvironmentCheckResult{Scope: "preflight", Code: code, Title: title, Severity: "ok", Summary: summary}
}

func chromiumReadyCheck(path string) EnvironmentCheckResult {
	return EnvironmentCheckResult{Scope: "preflight", Code: "chromium.ready", Title: "图片渲染 Chromium", Severity: "ok", Summary: "已找到可用浏览器。", Detail: path}
}

func workdirWritable(path string, writeProbe bool, previouslyWritable bool) bool {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) && writeProbe {
		if err = os.MkdirAll(path, 0o755); err == nil {
			info, err = os.Stat(path)
		}
	}
	if err != nil || !info.IsDir() {
		return false
	}
	if !writeProbe {
		return previouslyWritable
	}
	probe, err := os.CreateTemp(path, ".launcher-write-test-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	if _, err = probe.WriteString("ok"); err == nil {
		err = probe.Close()
	} else {
		probe.Close()
	}
	os.Remove(name)
	return err == nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
