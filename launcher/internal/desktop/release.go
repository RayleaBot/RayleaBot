package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

const releaseCacheTTL = 6 * time.Hour
const maxHelperOutput = 4 << 20

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

type buildInfo struct {
	Version    string `json:"version"`
	ArtifactID string `json:"artifact_id"`
}

type releaseCheck struct {
	Status           string `json:"status"`
	CurrentVersion   string `json:"current_version"`
	AvailableVersion string `json:"available_version"`
	ReleasePageURL   string `json:"release_page_url"`
}

type ReleaseFeed struct {
	mu       sync.Mutex
	basePath string
	cachedAt time.Time
	cached   ReleaseCheckSnapshot
}

func NewReleaseFeed(basePath string) *ReleaseFeed {
	return &ReleaseFeed{basePath: filepath.Clean(basePath), cached: releaseUnavailable("尚未检查版本。")}
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
	defer func() { r.cachedAt = time.Now() }()
	info, err := readBuildInfoForPlatform(r.basePath, goos, goarch)
	if err != nil {
		r.cached = releaseUnavailable("当前安装缺少有效版本信息，请打开发布页查看版本。")
		r.cached.ReleasePageURL = repositoryURL + "/releases/latest"
		return r.cached
	}
	serverName := "raylea-server"
	if goos == "windows" {
		serverName += ".exe"
	}
	stdout, _, err := runHelper(filepath.Join(r.basePath, serverName), "-config", filepath.Join(r.basePath, "config", "user.yaml"), "update", "check", "--json")
	if err != nil {
		r.cached = ReleaseCheckSnapshot{Status: "failed", CurrentVersion: info.Version, Summary: "检查更新失败。", Detail: "无法获取发布信息，请稍后重试或打开发布页。", ErrorCode: "launcher.update_check_failed", CanCheck: true, ReleasePageURL: repositoryURL + "/releases/latest"}
		return r.cached
	}
	result, err := parseReleaseCheck(stdout)
	if err != nil {
		r.cached = ReleaseCheckSnapshot{Status: "failed", CurrentVersion: info.Version, Summary: "发布信息无效。", Detail: "版本检查返回了无效的发布信息。", ErrorCode: "launcher.update_response_invalid", CanCheck: true}
		return r.cached
	}
	r.cached = snapshotFromCheck(result)
	return r.cached
}

func parseReleaseCheck(payload string) (releaseCheck, error) {
	var result releaseCheck
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return result, err
	}
	if result.Status != "up_to_date" && result.Status != "update_available" {
		return result, errors.New("invalid update status")
	}
	if !semverPattern.MatchString(result.CurrentVersion) || !semverPattern.MatchString(result.AvailableVersion) {
		return result, errors.New("invalid release version")
	}
	if !safeReleaseURL(result.ReleasePageURL) {
		return result, errors.New("invalid release page URL")
	}
	return result, nil
}

func snapshotFromCheck(result releaseCheck) ReleaseCheckSnapshot {
	available := result.Status == "update_available"
	summary := fmt.Sprintf("当前版本 %s 已是最新。", result.CurrentVersion)
	if available {
		summary = fmt.Sprintf("发现新版本 %s。", result.AvailableVersion)
	}
	return ReleaseCheckSnapshot{Status: ReleaseCheckStatus(result.Status), CurrentVersion: result.CurrentVersion, LatestVersion: result.AvailableVersion, Summary: summary, ReleasePageURL: result.ReleasePageURL, UpdateAvailable: available, CanCheck: true}
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
	if !semverPattern.MatchString(info.Version) || expectedArtifactID == "" || info.ArtifactID != expectedArtifactID {
		return buildInfo{}, errors.New("build_info.json 缺少有效的版本或平台信息")
	}
	return info, nil
}

func launcherArtifactID(goos, goarch string) string {
	switch {
	case goos == "windows" && goarch == "amd64":
		return ArtifactWindowsX64Full
	case goos == "linux" && goarch == "amd64":
		return ArtifactLinuxX64Full
	case goos == "darwin" && goarch == "arm64":
		return ArtifactMacOSARM64Full
	default:
		return ""
	}
}

func runHelper(executable string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	configureChildProcess(command)
	var stdout, stderr limitedBuffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = errors.New("版本检查超时")
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

func safeReleaseURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}
