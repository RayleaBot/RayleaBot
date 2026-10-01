package desktop

import (
	"encoding/hex"
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
		checkPath("launcher.settings", "launcher.settings_invalid", "启动器设置", settings.ServerExecutablePath != "" && settings.ConfigPath != "" && settings.Workdir != "", "启动器设置已解析。", "服务端程序、配置文件或工作目录未能解析。", "请在偏好设置中检查路径设置。", true),
		checkPath("server.executable", "server.executable_missing", "服务端程序", regularFile(settings.ServerExecutablePath), "已找到服务端程序。", fmt.Sprintf("未找到服务端程序：%s", settings.ServerExecutablePath), "请选择有效的服务端程序。", true),
	}

	userConfigExists := regularFile(settings.ConfigPath)
	switch {
	case userConfigExists:
		checks = append(checks, okCheck("config.file", "用户配置", "用户配置可用。"))
	case !pathExists(settings.ConfigPath) && regularFile(settings.ServerExecutablePath):
		// The Launcher generates the file before it starts the service, so a missing file needs no action.
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "config.bootstrap_available", Title: "用户配置", Severity: "ok",
			Summary: "尚未生成，启动服务时自动生成。", Detail: fmt.Sprintf("尚未找到 %s", settings.ConfigPath),
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
			Summary: "未找到运行环境清单。", Detail: fmt.Sprintf("检查路径：%s", manifestPath), Remediation: "请恢复当前版本附带的运行环境清单。",
		}}
		if browser := findSystemChromium(); browser != "" {
			checks = append(checks, chromiumReadyCheck(browser))
		}
		return checks
	}

	var manifest depsManifest
	if json.Unmarshal(payload, &manifest) != nil {
		return []EnvironmentCheckResult{{
			Scope: "preflight", Code: "deps.manifest_invalid", Title: "运行环境清单", Severity: "warning",
			Summary: "运行环境清单内容无效。", Detail: fmt.Sprintf("检查路径：%s", manifestPath), Remediation: "请恢复当前版本附带的运行环境清单。",
		}}
	}

	platform := manifestPlatform()
	platformName := platformLabel(runtime.GOOS, runtime.GOARCH)
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
		checks = append(checks, okCheck("deps.manifest", "运行环境清单", fmt.Sprintf("已包含当前平台资源：%s", platformName)))
	} else {
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "deps.manifest_platform_missing", Title: "运行环境清单", Severity: "warning",
			Summary: "运行环境清单缺少当前平台资源。", Detail: fmt.Sprintf("平台：%s", platformName), Remediation: "请恢复当前平台的运行环境资源。",
		})
	}
	if chromium == nil {
		if browser := findSystemChromium(); browser != "" {
			checks = append(checks, chromiumReadyCheck(browser))
		} else {
			checks = append(checks, EnvironmentCheckResult{
				Scope: "preflight", Code: "chromium.resource_missing", Title: "图片渲染 Chromium", Severity: "warning",
				Summary: "运行环境清单中没有 Chromium。", Remediation: "请恢复当前版本附带的运行环境清单，或安装 Chrome、Edge 或 Chromium。",
			})
		}
	} else if !resourceMetadataComplete(*chromium) {
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "chromium.metadata_incomplete", Title: "图片渲染 Chromium", Severity: "warning",
			Summary: "运行环境清单中的 Chromium 信息不完整。", Remediation: "请恢复当前版本附带的运行环境清单。",
		})
	} else {
		checks = append(checks, inspectChromiumState(root, *chromium, findSystemChromium))
	}
	if ffmpeg == nil {
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "ffmpeg.resource_missing", Title: "媒体工具 FFmpeg", Severity: "warning",
			Summary: "运行环境清单中没有 FFmpeg。", Remediation: "请恢复当前版本附带的运行环境清单。",
		})
	} else if !resourceMetadataComplete(*ffmpeg) {
		checks = append(checks, EnvironmentCheckResult{
			Scope: "preflight", Code: "ffmpeg.metadata_incomplete", Title: "媒体工具 FFmpeg", Severity: "warning",
			Summary: "运行环境清单中的 FFmpeg 信息不完整。", Remediation: "请恢复当前版本附带的运行环境清单。",
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
	code, summary, detail := pendingRuntimeState(root, chromium, storeRoot)
	return EnvironmentCheckResult{Scope: "preflight", Code: "chromium." + code, Title: "图片渲染 Chromium", Severity: "ok", Summary: summary, Detail: detail}
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
	code, summary, detail := pendingRuntimeState(root, ffmpeg, storeRoot)
	return EnvironmentCheckResult{Scope: "preflight", Code: "ffmpeg." + code, Title: "媒体工具 FFmpeg", Severity: "ok", Summary: summary, Detail: detail}
}

// pendingRuntimeState describes a declared resource that is not prepared yet. The service prepares it each
// time it starts, so these states pass the preflight; the code suffix keeps the exact state for diagnostics.
func pendingRuntimeState(root string, resource depsResource, storeRoot string) (code, summary, detail string) {
	archivePath := filepath.Join(root, "cache", "downloads", "runtime", resource.ID+"-"+resource.Version+runtimeArchiveSuffix(resource.ArchiveFormat))
	tempRoots := findRuntimeTempRoots(filepath.Dir(storeRoot), resource.ID, resource.Version)
	switch {
	case len(tempRoots) > 0 && !pathExists(storeRoot):
		return "extract_incomplete", "上次解压未完成，启动服务时自动重新准备。", fmt.Sprintf("下载位置：%s。解压位置：%s。临时目录：%s", archivePath, storeRoot, strings.Join(tempRoots, "、"))
	case pathExists(storeRoot):
		return "entrypoint_missing", "文件不完整，启动服务时自动重新准备。", fmt.Sprintf("运行时目录：%s", storeRoot)
	case regularFile(archivePath):
		return "not_ready", "已下载，启动服务时自动解压。", fmt.Sprintf("下载位置：%s。解压位置：%s", archivePath, storeRoot)
	default:
		return "not_ready", "尚未准备，启动服务时自动准备。", fmt.Sprintf("运行时目录：%s", storeRoot)
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

// resourceMetadataComplete checks the fields the preflight reads. The Server
// validates the full manifest before it prepares a resource.
func resourceMetadataComplete(resource depsResource) bool {
	required := map[string][]string{"chromium": {"browser"}, "ffmpeg": {"ffmpeg", "ffprobe"}}[resource.Kind]
	if _, err := hex.DecodeString(resource.SHA256); err != nil || len(resource.SHA256) != 64 {
		return false
	}
	if resource.ID == "" || resource.Version == "" || resource.ArchiveFormat == "" || len(resource.Sources) == 0 || len(required) == 0 {
		return false
	}
	for _, source := range resource.Sources {
		if !strings.HasPrefix(source.URL, "https://") {
			return false
		}
	}
	for _, name := range required {
		if len(resource.Entrypoints[name]) == 0 {
			return false
		}
		for _, candidate := range resource.Entrypoints[name] {
			if !validRelativeEntrypoint(candidate) {
				return false
			}
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

// platformLabel names the platform for display only; manifestPlatform stays the identifier resources match.
func platformLabel(goos, goarch string) string {
	if goos == "darwin" && goarch == "arm64" {
		return "macOS（Apple 芯片）"
	}
	system := goos
	switch goos {
	case "windows":
		system = "Windows"
	case "linux":
		system = "Linux"
	case "darwin":
		system = "macOS"
	}
	architecture := goarch
	switch goarch {
	case "amd64":
		architecture = "x64"
	case "386":
		architecture = "x86"
	case "arm64":
		architecture = "ARM64"
	}
	return system + " " + architecture
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
	severity := CheckWarning
	if blocking {
		severity = CheckError
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
