package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
)

func runVersion(cmd Command) int {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "output build information as JSON")
	if err := flags.Parse(cmd.Args); err != nil || !*jsonOutput || flags.NArg() != 0 {
		cmd.Logger.Error("版本命令参数无效，用法 raylea version --json")
		return 1
	}
	repoRoot := runtimepaths.RootFromConfigPath(cmd.ConfigPath)
	payload, err := os.ReadFile(filepath.Join(repoRoot, "build_info.json"))
	if err != nil {
		cmd.Logger.Error("读取构建信息失败", "err", err.Error())
		return 1
	}
	buildInfo, err := releaseupdate.DecodeBuildInfo(payload)
	if err != nil {
		cmd.Logger.Error("构建信息无效", "err", err.Error())
		return 1
	}
	return writeCommandJSON(cmd, struct {
		Version    string `json:"version"`
		GitCommit  string `json:"git_commit"`
		ArtifactID string `json:"artifact_id"`
	}{
		Version:    buildInfo.Version,
		GitCommit:  buildInfo.GitCommit,
		ArtifactID: buildInfo.ArtifactID,
	})
}

func runUpdate(cmd Command) int {
	if len(cmd.Args) == 0 {
		cmd.Logger.Error("更新命令缺少子命令，用法 raylea update check --json | download | apply")
		return 1
	}
	child := cmd
	child.Args = cmd.Args[1:]
	switch cmd.Args[0] {
	case "check":
		return runUpdateCheck(child)
	case "download":
		return runUpdateDownload(child)
	case "apply":
		return runLifecycleLocked(child, "离线更新", runUpdateApply)
	default:
		cmd.Logger.Error("未知更新子命令", "subcommand", cmd.Args[0])
		return 1
	}
}

func newUpdateChecker(cmd Command) (*releaseupdate.Checker, error) {
	checker := releaseupdate.NewChecker()
	cfg, _, err := config.Load(cmd.ConfigPath, cmd.SchemaPath)
	if err != nil {
		return nil, err
	}
	checker.Settings = cfg.Update
	if cmd.UpdateHTTPClient != nil {
		checker.HTTPClient = cmd.UpdateHTTPClient
		checker.DownloadClient = cmd.UpdateHTTPClient
	}
	return checker, nil
}

func runUpdateCheck(cmd Command) int {
	flags := flag.NewFlagSet("update check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "output update status as JSON")
	if err := flags.Parse(cmd.Args); err != nil || !*jsonOutput || flags.NArg() != 0 {
		cmd.Logger.Error("更新检查参数无效，用法 raylea update check --json")
		return 1
	}
	repoRoot := runtimepaths.RootFromConfigPath(cmd.ConfigPath)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	checker, err := newUpdateChecker(cmd)
	if err != nil {
		cmd.Logger.Error("读取更新设置失败", "err", displayLogError(repoRoot, err))
		return 1
	}
	result, err := checker.Check(ctx, repoRoot)
	if err != nil {
		cmd.Logger.Error("检查更新失败", "code", releaseupdate.CodeOf(err), "err", err.Error())
		return 1
	}
	return writeCommandJSON(cmd, struct {
		Status           string `json:"status"`
		CurrentVersion   string `json:"current_version"`
		AvailableVersion string `json:"available_version"`
		UpdateMode       string `json:"update_mode"`
		ReleasePageURL   string `json:"release_page_url"`
	}{
		Status:           result.Status,
		CurrentVersion:   result.CurrentVersion,
		AvailableVersion: result.AvailableVersion,
		UpdateMode:       result.UpdateMode,
		ReleasePageURL:   result.ReleasePageURL,
	})
}

func runUpdateDownload(cmd Command) int {
	flags := flag.NewFlagSet("update download", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	file := flags.String("file", "", "prepare a local release archive")
	progressJSON := flags.Bool("progress-json", false, "emit update progress as JSONL")
	if flags.Parse(cmd.Args) != nil || flags.NArg() != 0 {
		cmd.Logger.Error("更新下载参数无效，用法 raylea update download")
		return 1
	}
	repoRoot := runtimepaths.RootFromConfigPath(cmd.ConfigPath)
	checker, err := newUpdateChecker(cmd)
	if err != nil {
		cmd.Logger.Error("读取更新设置失败", "err", displayLogError(repoRoot, err))
		return 1
	}
	downloadContext, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	var result releaseupdate.CheckResult
	encoder := json.NewEncoder(commandStdout(cmd))
	if *progressJSON {
		checker.Progress = func(p releaseupdate.Progress) { _ = encoder.Encode(p) }
	}
	var archivePath string
	if *file != "" {
		result, err = checker.Import(downloadContext, repoRoot, *file)
	} else {
		result, archivePath, err = checker.Download(downloadContext, repoRoot)
	}
	if err != nil {
		cmd.Logger.Error("下载更新失败", "code", releaseupdate.CodeOf(err), "err", displayLogError(repoRoot, err))
		return 1
	}
	if result.Status != "update_available" {
		if *progressJSON {
			_ = encoder.Encode(map[string]any{"stage": "prepared", "status": result.Status, "version": result.CurrentVersion, "prepared_id": ""})
		}
		cmd.Logger.Info("当前已是最新版本", "current_version", result.CurrentVersion)
		return 0
	}
	cmd.Logger.Info("更新包已下载", "current_version", result.CurrentVersion,
		"available_version", result.AvailableVersion, "archive_path", displayLogPath(repoRoot, archivePath))
	if *progressJSON {
		_ = encoder.Encode(map[string]any{"stage": "prepared", "status": result.Status, "version": result.AvailableVersion, "prepared_id": result.PreparedID})
	}
	return 0
}

func runUpdateApply(cmd Command) int {
	flags := flag.NewFlagSet("update apply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	preparedID := flags.String("prepared", "", "require a specific preparation")
	if flags.Parse(cmd.Args) != nil || flags.NArg() != 0 {
		cmd.Logger.Error("更新安装参数无效，用法 raylea update apply")
		return 1
	}
	repoRoot := runtimepaths.RootFromConfigPath(cmd.ConfigPath)
	result, err := releaseupdate.NewChecker().ApplyPrepared(context.Background(), repoRoot, *preparedID)
	if err != nil {
		cmd.Logger.Error("安装更新失败", "code", releaseupdate.CodeOf(err), "err", displayLogError(repoRoot, err))
		return 1
	}
	if result.Status != "update_available" {
		cmd.Logger.Info("当前已是最新版本", "current_version", result.CurrentVersion)
		return 0
	}
	cmd.Logger.Info("更新已安装，重新启动服务后生效", "previous_version", result.CurrentVersion, "version", result.AvailableVersion)
	return 0
}

func writeCommandJSON(cmd Command, value any) int {
	encoder := json.NewEncoder(commandStdout(cmd))
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		cmd.Logger.Error("写入 JSON 输出失败", "err", err.Error())
		return 1
	}
	return 0
}
