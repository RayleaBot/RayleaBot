package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

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
		cmd.Logger.Error("更新命令缺少子命令，用法 raylea update check --json")
		return 1
	}
	child := cmd
	child.Args = cmd.Args[1:]
	switch cmd.Args[0] {
	case "check":
		return runUpdateCheck(child)
	default:
		cmd.Logger.Error("未知更新子命令", "subcommand", cmd.Args[0])
		return 1
	}
}

func runUpdateCheck(cmd Command) int {
	flags := flag.NewFlagSet("update check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "output update status as JSON")
	if err := flags.Parse(cmd.Args); err != nil || !*jsonOutput || flags.NArg() != 0 {
		cmd.Logger.Error("更新检查参数无效，用法 raylea update check --json")
		return 1
	}
	checker := releaseupdate.NewChecker()
	if cmd.UpdateHTTPClient != nil {
		checker.HTTPClient = cmd.UpdateHTTPClient
	}
	repoRoot := runtimepaths.RootFromConfigPath(cmd.ConfigPath)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
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

func writeCommandJSON(cmd Command, value any) int {
	encoder := json.NewEncoder(commandStdout(cmd))
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		cmd.Logger.Error("写入 JSON 输出失败", "err", err.Error())
		return 1
	}
	return 0
}
