package desktop

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/launcher/internal/contractversions"
)

const (
	releaseCacheTTL = 6 * time.Hour
	helperTimeout   = 30 * time.Minute
	maxHelperOutput = 4 << 20
)

var (
	semverPattern       = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	artifactNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$`)
)

type buildInfo struct {
	Version               string `json:"version"`
	ArtifactID            string `json:"artifact_id"`
	UpdateProtocolVersion int    `json:"update_protocol_version"`
}

type trustedArtifact struct {
	ArtifactID       string `json:"artifact_id"`
	FileName         string `json:"file_name"`
	ArchiveSizeBytes int64  `json:"archive_size_bytes"`
	UpdateMode       string `json:"update_mode"`
}

type trustedCheck struct {
	Status                    string          `json:"status"`
	CurrentVersion            string          `json:"current_version"`
	AvailableVersion          string          `json:"available_version"`
	UpdateMode                string          `json:"update_mode"`
	ReleasePageURL            string          `json:"release_page_url"`
	AutomaticInstallSupported bool            `json:"automatic_install_supported"`
	ManifestPath              string          `json:"manifest_path"`
	SignaturePath             string          `json:"signature_path"`
	Artifact                  trustedArtifact `json:"artifact"`
	ArtifactPath              string          `json:"artifact_path"`
}

type ReleaseFeed struct {
	mu sync.Mutex

	basePath    string
	updaterPath string
	cachedAt    time.Time
	cached      ReleaseCheckSnapshot
	checked     *trustedCheck
	downloaded  *trustedCheck
}

type releaseFailureContext struct {
	installRoot     string
	updaterPath     string
	transactionRoot string
}

func NewReleaseFeed(basePath string) *ReleaseFeed {
	return &ReleaseFeed{
		basePath:    filepath.Clean(basePath),
		updaterPath: filepath.Join(basePath, "raylea-updater.exe"),
		cached:      releaseUnavailable("尚未检查版本。"),
	}
}

func (r *ReleaseFeed) GetSnapshot(force bool) ReleaseCheckSnapshot {
	return r.getSnapshot(force, runtime.GOOS, runtime.GOARCH)
}

func (r *ReleaseFeed) getSnapshot(force bool, goos, goarch string) ReleaseCheckSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !force && !r.cachedAt.IsZero() && time.Since(r.cachedAt) < releaseCacheTTL {
		return r.cached
	}
	info, err := readBuildInfoForPlatform(r.basePath, goos, goarch)
	if err != nil {
		r.cached = r.failure("disabled", "release.trust_required", "当前安装缺少自动更新所需的可信签名信息。", err.Error(), "")
		r.cachedAt = time.Now()
		r.checked, r.downloaded = nil, nil
		return r.cached
	}
	if goos != "windows" || goarch != "amd64" {
		r.cached = releaseUnavailable("当前平台采用发布页引导更新；自动安装仅支持 Windows x64 整包。")
		r.cached.CurrentVersion = info.Version
		r.cached.ReleasePageURL = repositoryURL + "/releases/latest"
		r.cachedAt = time.Now()
		r.checked, r.downloaded = nil, nil
		return r.cached
	}
	if err := assertRegularFile(r.updaterPath); err != nil {
		r.cached = r.failure("failed", "launcher.update_helper_missing", "更新助手不存在。", err.Error(), info.Version)
		r.cached.CanCheck = true
		r.cachedAt = time.Now()
		return r.cached
	}
	stdout, stderr, err := runHelper(r.updaterPath, "check", "--install-root", r.basePath, "--json")
	if err != nil {
		failureCode, detail := parseHelperFailure(stderr, err)
		r.cached = r.failure("failed", failureCode, releaseErrorSummary(failureCode), detail, info.Version)
		r.cached.CanCheck = true
		r.cachedAt = time.Now()
		r.checked, r.downloaded = nil, nil
		return r.cached
	}
	result, err := parseWindowsUpdaterCheck(stdout, false)
	if err != nil {
		r.cached = r.failure("failed", "launcher.update_response_invalid", "更新助手返回的检查结果无效。", err.Error(), info.Version)
		r.cached.CanCheck = true
		r.cachedAt = time.Now()
		return r.cached
	}
	r.checked = &result
	r.downloaded = nil
	r.cached = snapshotFromCheck(result)
	r.cachedAt = time.Now()
	return r.cached
}

func (r *ReleaseFeed) Download() ReleaseCheckSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.checked == nil || r.checked.Status != "update_available" || !r.checked.AutomaticInstallSupported || r.checked.UpdateMode != "automatic" {
		return r.cached
	}
	previous := r.cached
	stdout, stderr, err := runHelper(r.updaterPath, "download", "--install-root", r.basePath, "--json")
	if err != nil {
		code, detail := parseHelperFailure(stderr, err)
		r.cached = r.failure("failed", code, releaseErrorSummary(code), detail, previous.CurrentVersion)
		r.cached.LatestVersion = previous.LatestVersion
		r.cached.ReleasePageURL = previous.ReleasePageURL
		r.cached.CanCheck, r.cached.CanDownload = true, true
		return r.cached
	}
	result, err := parseWindowsUpdaterCheck(stdout, true)
	if err != nil {
		r.cached = r.failure("failed", "launcher.update_response_invalid", "更新助手返回的下载结果无效。", err.Error(), previous.CurrentVersion)
		r.cached.CanCheck, r.cached.CanDownload = true, true
		return r.cached
	}
	if !result.AutomaticInstallSupported || result.UpdateMode != "automatic" {
		r.cached = r.failure("failed", "release.update_not_supported", "当前发布不支持自动安装。", "受信任发布清单未授权自动安装此更新包。", result.CurrentVersion)
		return r.cached
	}
	progress := float64(1)
	downloadedBytes := result.Artifact.ArchiveSizeBytes
	r.checked = &result
	r.downloaded = &result
	r.cached = ReleaseCheckSnapshot{
		Status: "ready_to_install", CurrentVersion: result.CurrentVersion, LatestVersion: result.AvailableVersion,
		Summary: fmt.Sprintf("新版本 %s 已验证并准备安装。", result.AvailableVersion),
		Detail:  "确认安装后会停服、离线备份、原子替换，并在失败时自动回滚。", ReleasePageURL: result.ReleasePageURL, UpdateAvailable: true,
		DownloadProgress: &progress, DownloadedBytes: &downloadedBytes, TotalBytes: &downloadedBytes, ArtifactFileName: result.Artifact.FileName,
		CanCheck: true, CanInstall: true,
	}
	return r.cached
}

func (r *ReleaseFeed) Install(launcherPID int, serviceWasRunning bool, settings LauncherResolvedSettings) (ReleaseCheckSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if runtime.GOOS != "windows" || r.downloaded == nil || !r.downloaded.AutomaticInstallSupported {
		r.cached = r.installFailure("failed", "release.update_not_supported", "没有可自动安装的受信任更新。", "请重新检查并下载更新，或按发布页指引手动升级。", r.cached.CurrentVersion)
		return r.cached, errors.New(r.cached.Detail)
	}
	officialServer := filepath.Join(r.basePath, "raylea-server.exe")
	officialConfig := filepath.Join(r.basePath, "config", "user.yaml")
	if !samePath(settings.InstallationRoot, r.basePath) || !samePath(settings.ServerExecutablePath, officialServer) || !samePath(settings.ConfigPath, officialConfig) {
		r.cached = r.installFailure("failed", "release.update_not_supported", "当前运行方式不支持自动安装。", "检测到自定义安装根、服务程序或配置路径，请按发布页指引手动升级。", r.cached.CurrentVersion)
		return r.cached, errors.New(r.cached.Detail)
	}
	downloaded := *r.downloaded
	randomSuffix, err := randomHex(8)
	if err != nil {
		r.cached = r.installFailure("failed", "release.install_failed", "无法准备更新事务。", err.Error(), r.cached.CurrentVersion)
		return r.cached, errors.New(r.cached.Detail)
	}
	transactionRoot := filepath.Join(filepath.Dir(r.basePath), fmt.Sprintf(".rayleabot-update-%d-%s", time.Now().UnixMilli(), randomSuffix))
	if !transactionSibling(r.basePath, transactionRoot) {
		return r.cached, errors.New("更新事务目录不在受控位置")
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(transactionRoot)
		}
	}()
	for _, source := range []string{r.updaterPath, downloaded.ManifestPath, downloaded.SignaturePath, downloaded.ArtifactPath} {
		if err := assertRegularFile(source); err != nil {
			r.cached = r.installFailure("failed", "launcher.update_input_missing", "更新事务输入不存在。", err.Error(), r.cached.CurrentVersion, transactionRoot)
			return r.cached, errors.New(r.cached.Detail)
		}
	}
	cacheRoot := filepath.Join(r.basePath, "cache", "downloads", "updates")
	for _, source := range []string{downloaded.ManifestPath, downloaded.SignaturePath, downloaded.ArtifactPath} {
		if !pathInside(cacheRoot, source) {
			err := errors.New("发布清单、签名或更新包位于受控下载缓存之外")
			r.cached = r.installFailure("failed", "launcher.update_input_outside_cache", "更新事务输入位置无效。", err.Error(), r.cached.CurrentVersion, transactionRoot)
			return r.cached, errors.New(r.cached.Detail)
		}
	}
	if err := os.Mkdir(transactionRoot, 0o700); err != nil {
		r.cached = r.installFailure("failed", "release.install_failed", "无法创建更新事务目录。", err.Error(), r.cached.CurrentVersion, transactionRoot)
		return r.cached, errors.New(r.cached.Detail)
	}
	externalHelper := filepath.Join(transactionRoot, "raylea-updater.exe")
	manifestPath := filepath.Join(transactionRoot, "release_manifest.v2.json")
	signaturePath := filepath.Join(transactionRoot, "release_manifest.v2.sig.json")
	artifactPath := filepath.Join(transactionRoot, downloaded.Artifact.FileName)
	for source, destination := range map[string]string{
		r.updaterPath: externalHelper, downloaded.ManifestPath: manifestPath, downloaded.SignaturePath: signaturePath, downloaded.ArtifactPath: artifactPath,
	} {
		if err := copyExclusive(source, destination); err != nil {
			r.cached = r.installFailure("failed", "release.install_failed", "无法准备更新事务输入。", err.Error(), r.cached.CurrentVersion, transactionRoot)
			return r.cached, errors.New(r.cached.Detail)
		}
	}
	command := exec.Command(externalHelper,
		"install", "--install-root", r.basePath, "--transaction-root", transactionRoot,
		"--manifest", manifestPath, "--signature", signaturePath, "--artifact", artifactPath,
		"--launcher-pid", fmt.Sprint(launcherPID), fmt.Sprintf("--service-was-running=%t", serviceWasRunning),
	)
	configureDetachedProcess(command)
	command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
	if err := command.Start(); err != nil {
		r.cached = r.installFailure("failed", "release.install_failed", "无法启动更新助手。", err.Error(), r.cached.CurrentVersion, transactionRoot)
		return r.cached, errors.New(r.cached.Detail)
	}
	cleanup = false
	r.cached.Status = "installing"
	r.cached.Summary = "正在准备事务式安装。"
	r.cached.Detail = "启动器退出后，外置更新助手会完成备份、替换、健康检查和必要的回滚。"
	r.cached.ErrorCode = ""
	r.cached.CanCheck, r.cached.CanDownload, r.cached.CanInstall = false, false, false
	return r.cached, nil
}

func readBuildInfo(basePath string) (buildInfo, error) {
	return readBuildInfoForPlatform(basePath, runtime.GOOS, runtime.GOARCH)
}

func readBuildInfoForPlatform(basePath, goos, goarch string) (buildInfo, error) {
	payload, err := os.ReadFile(filepath.Join(basePath, "build_info.json"))
	if err != nil {
		return buildInfo{}, fmt.Errorf("读取 build_info.json: %w", err)
	}
	var info buildInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		return buildInfo{}, fmt.Errorf("解析 build_info.json: %w", err)
	}
	expectedArtifactID := launcherArtifactID(goos, goarch)
	if !semverPattern.MatchString(info.Version) || expectedArtifactID == "" || info.ArtifactID != expectedArtifactID || info.UpdateProtocolVersion < contractversions.UpdateProtocolVersion {
		return buildInfo{}, errors.New("build_info.json 未提供可受信任的自动更新基线")
	}
	return info, nil
}

func launcherArtifactID(goos, goarch string) string {
	switch {
	case goos == "windows" && goarch == "amd64":
		return "windows-x64-full"
	case goos == "linux" && goarch == "amd64":
		return "linux-x64-full"
	case goos == "darwin" && goarch == "arm64":
		return "macos-arm64-full"
	default:
		return ""
	}
}

func parseWindowsUpdaterCheck(payload string, requireArtifactPath bool) (trustedCheck, error) {
	var result trustedCheck
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return result, err
	}
	if result.Status != "up_to_date" && result.Status != "update_available" {
		return result, errors.New("status 无效")
	}
	if !semverPattern.MatchString(result.CurrentVersion) || (result.Status == "update_available" && !semverPattern.MatchString(result.AvailableVersion)) {
		return result, errors.New("版本号不是有效语义版本")
	}
	if result.UpdateMode != "automatic" && result.UpdateMode != "guided" && result.UpdateMode != "manual" {
		return result, errors.New("update_mode 无效")
	}
	if result.Artifact.ArtifactID != "windows-x64-full" || !safeArtifactName(result.Artifact.FileName) || result.Artifact.ArchiveSizeBytes <= 0 || result.Artifact.ArchiveSizeBytes > 2<<30 || result.Artifact.UpdateMode != result.UpdateMode {
		return result, errors.New("artifact 信息无效")
	}
	if result.ReleasePageURL != "" && !safeReleaseURL(result.ReleasePageURL) {
		return result, errors.New("release_page_url 必须是无凭据的 HTTPS 地址")
	}
	if result.AutomaticInstallSupported && result.UpdateMode != "automatic" {
		return result, errors.New("automatic_install_supported 与 update_mode 冲突")
	}
	if requireArtifactPath && strings.TrimSpace(result.ArtifactPath) == "" {
		return result, errors.New("下载结果缺少 artifact_path")
	}
	return result, nil
}

func snapshotFromCheck(result trustedCheck) ReleaseCheckSnapshot {
	available := result.Status == "update_available"
	automatic := available && result.UpdateMode == "automatic" && result.AutomaticInstallSupported
	summary := fmt.Sprintf("当前版本 %s 已是最新。", result.CurrentVersion)
	detail := ""
	latest := result.CurrentVersion
	if available {
		latest = result.AvailableVersion
		summary = fmt.Sprintf("发现新版本 %s。", latest)
		if automatic {
			detail = "确认后才会下载和安装；安装前后均执行完整信任校验。"
		} else {
			detail = "此发布仅提供引导更新，请打开发布页按说明手动升级。"
		}
	}
	total := result.Artifact.ArchiveSizeBytes
	return ReleaseCheckSnapshot{
		Status: ReleaseCheckStatus(result.Status), CurrentVersion: result.CurrentVersion, LatestVersion: latest, Summary: summary, Detail: detail,
		ReleasePageURL: result.ReleasePageURL, UpdateAvailable: available, TotalBytes: &total, ArtifactFileName: result.Artifact.FileName,
		CanCheck: true, CanDownload: automatic,
	}
}

func (r *ReleaseFeed) failure(status ReleaseCheckStatus, code, summary, detail, currentVersion string, transactionRoot ...string) ReleaseCheckSnapshot {
	context := releaseFailureContext{installRoot: r.basePath, updaterPath: r.updaterPath}
	if len(transactionRoot) > 0 {
		context.transactionRoot = transactionRoot[0]
	}
	return ReleaseCheckSnapshot{
		Status: status, CurrentVersion: currentVersion, Summary: summary,
		Detail: sanitizeReleaseDetail(detail, context), ErrorCode: code,
	}
}

func (r *ReleaseFeed) installFailure(status ReleaseCheckStatus, code, summary, detail, currentVersion string, transactionRoot ...string) ReleaseCheckSnapshot {
	previous := r.cached
	failed := r.failure(status, code, summary, detail, currentVersion, transactionRoot...)
	failed.LatestVersion = previous.LatestVersion
	failed.ReleasePageURL = previous.ReleasePageURL
	failed.UpdateAvailable = previous.UpdateAvailable
	failed.DownloadProgress = previous.DownloadProgress
	failed.DownloadedBytes = previous.DownloadedBytes
	failed.TotalBytes = previous.TotalBytes
	failed.ArtifactFileName = previous.ArtifactFileName
	failed.CanCheck = true
	failed.CanInstall = r.downloaded != nil && r.downloaded.AutomaticInstallSupported
	return failed
}

func releaseErrorSummary(code string) string {
	switch code {
	case "release.trust_required":
		return "当前安装不具备自动更新信任基线。"
	case "release.manifest_invalid":
		return "发布清单无效。"
	case "release.signature_invalid":
		return "发布签名验证失败。"
	case "release.manifest_expired":
		return "发布清单已过期。"
	case "release.replay_rejected":
		return "已拒绝旧版或被替换的发布清单。"
	case "release.artifact_invalid":
		return "更新包完整性或签名验证失败。"
	case "release.update_not_supported":
		return "当前平台仅支持引导更新。"
	case "release.disk_space_insufficient":
		return "磁盘空间不足。"
	case "release.install_failed":
		return "更新安装失败。"
	case "release.rollback_failed":
		return "更新回滚失败。"
	default:
		return "更新助手执行失败。"
	}
}

func runHelper(executable string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	configureChildProcess(command)
	var stdout, stderr limitedBuffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = errors.New("更新助手执行超时")
	}
	return stdout.String(), stderr.String(), err
}

type limitedBuffer struct {
	buffer bytes.Buffer
}

func (b *limitedBuffer) Write(payload []byte) (int, error) {
	original := len(payload)
	remaining := maxHelperOutput - b.buffer.Len()
	if remaining <= 0 {
		return original, nil
	}
	if len(payload) > remaining {
		payload = payload[:remaining]
	}
	_, _ = b.buffer.Write(payload)
	return original, nil
}

func (b *limitedBuffer) String() string { return b.buffer.String() }

func parseHelperFailure(stderr string, fallback error) (string, string) {
	var payload struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload) == nil && (payload.Code != "" || payload.Error != "") {
		return firstNonEmpty(payload.Code, "launcher.update_helper_failed"), firstNonEmpty(payload.Error, fallback.Error())
	}
	return "launcher.update_helper_failed", fallback.Error()
}

func assertRegularFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("文件路径为空")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s 不是普通文件", path)
	}
	return nil
}

func copyExclusive(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func safeArtifactName(value string) bool {
	return artifactNamePattern.MatchString(value)
}

func safeReleaseURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func sanitizeReleaseDetail(value string, context releaseFailureContext) string {
	value = strings.TrimSpace(strings.Map(func(character rune) rune {
		if character < 32 && character != '\n' && character != '\t' {
			return -1
		}
		return character
	}, value))
	value = redactKnownPath(value, context.transactionRoot, "<更新事务目录>")
	value = redactKnownPath(value, context.updaterPath, "raylea-updater.exe")
	value = redactKnownPath(value, context.installRoot, "<安装目录>")
	characters := []rune(value)
	if len(characters) > 2000 {
		value = strings.TrimSpace(string(characters[:1997])) + "..."
	}
	return value
}

func redactKnownPath(value, candidate, replacement string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return value
	}
	cleaned := absoluteClean(candidate)
	parts := strings.FieldsFunc(cleaned, func(character rune) bool {
		return character == '/' || character == '\\'
	})
	if len(parts) == 0 {
		return value
	}
	for index := range parts {
		parts[index] = regexp.QuoteMeta(parts[index])
	}
	pattern := strings.Join(parts, `[\\/]+`)
	if strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, `\`) {
		pattern = `[\\/]+` + pattern
	}
	matcher, err := regexp.Compile(`(?i)` + pattern)
	if err != nil {
		return value
	}
	return matcher.ReplaceAllString(value, replacement)
}

func transactionSibling(installRoot, transactionRoot string) bool {
	return samePath(filepath.Dir(absoluteClean(installRoot)), filepath.Dir(absoluteClean(transactionRoot))) && strings.HasPrefix(filepath.Base(transactionRoot), ".rayleabot-update-")
}

func pathInside(root, candidate string) bool {
	relative, err := filepath.Rel(absoluteClean(root), absoluteClean(candidate))
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func randomHex(size int) (string, error) {
	payload := make([]byte, size)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(payload), nil
}
