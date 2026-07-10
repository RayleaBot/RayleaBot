package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
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
	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
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
		Version               string `json:"version"`
		GitCommit             string `json:"git_commit"`
		ArtifactID            string `json:"artifact_id"`
		UpdateProtocolVersion int    `json:"update_protocol_version"`
	}{
		Version:               buildInfo.Version,
		GitCommit:             buildInfo.GitCommit,
		ArtifactID:            buildInfo.ArtifactID,
		UpdateProtocolVersion: buildInfo.UpdateProtocolVersion,
	})
}

func runUpdate(cmd Command) int {
	if len(cmd.Args) == 0 {
		cmd.Logger.Error("更新命令缺少子命令，用法 raylea update check --json 或 raylea update verify ...")
		return 1
	}
	child := cmd
	child.Args = cmd.Args[1:]
	switch cmd.Args[0] {
	case "check":
		return runUpdateCheck(child)
	case "verify":
		return runUpdateVerify(child)
	default:
		cmd.Logger.Error("未知更新子命令：" + cmd.Args[0])
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
	verifier, err := commandUpdateVerifier(cmd)
	if err != nil {
		cmd.Logger.Error("发布信任基线不可用", "code", releaseupdate.CodeOf(err), "err", err.Error())
		return 1
	}
	checker := releaseupdate.NewChecker(verifier)
	if cmd.UpdateHTTPClient != nil {
		checker.HTTPClient = cmd.UpdateHTTPClient
	}
	if cmd.Now != nil {
		checker.Now = cmd.Now
	}
	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := checker.Check(ctx, repoRoot)
	if err != nil {
		cmd.Logger.Error("检查受信任更新失败", "code", releaseupdate.CodeOf(err), "err", err.Error())
		return 1
	}
	return writeCommandJSON(cmd, struct {
		Status           string `json:"status"`
		CurrentVersion   string `json:"current_version"`
		AvailableVersion string `json:"available_version"`
		UpdateMode       string `json:"update_mode"`
	}{
		Status:           result.Status,
		CurrentVersion:   result.CurrentVersion,
		AvailableVersion: result.AvailableVersion,
		UpdateMode:       result.UpdateMode,
	})
}

func runUpdateVerify(cmd Command) int {
	flags := flag.NewFlagSet("update verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "", "path to release manifest")
	signaturePath := flags.String("signature", "", "path to signature envelope")
	artifactPath := flags.String("artifact", "", "path to release artifact")
	if err := flags.Parse(cmd.Args); err != nil || flags.NArg() != 0 || *manifestPath == "" || *signaturePath == "" || *artifactPath == "" {
		cmd.Logger.Error("更新验证参数无效，用法 raylea update verify --manifest <path> --signature <path> --artifact <path>")
		return 1
	}
	verifier, err := commandUpdateVerifier(cmd)
	if err != nil {
		cmd.Logger.Error("发布信任基线不可用", "code", releaseupdate.CodeOf(err), "err", err.Error())
		return 1
	}
	now := time.Now().UTC()
	if cmd.Now != nil {
		now = cmd.Now().UTC()
	}
	verified, err := releaseupdate.ValidateMetadataFiles(verifier, *manifestPath, *signaturePath, now)
	if err != nil {
		cmd.Logger.Error("发布元数据验证失败", "code", releaseupdate.CodeOf(err), "err", err.Error())
		return 1
	}
	artifactBase := filepath.Base(*artifactPath)
	var artifact releaseupdate.Artifact
	found := false
	for _, candidate := range verified.Manifest.Artifacts {
		if candidate.FileName == artifactBase {
			artifact = candidate
			found = true
			break
		}
	}
	if !found {
		cmd.Logger.Error("发布包未列入受信任清单", "code", releaseupdate.CodeArtifactInvalid)
		return 1
	}
	if err := releaseupdate.VerifyArtifactFile(*artifactPath, artifact); err != nil {
		cmd.Logger.Error("发布包验证失败", "code", releaseupdate.CodeOf(err), "err", err.Error())
		return 1
	}
	cmd.Logger.Info("发布清单、签名和更新包验证通过", "version", verified.Manifest.Version, "artifact_id", artifact.ArtifactID)
	return 0
}

func commandUpdateVerifier(cmd Command) (*releaseupdate.Verifier, error) {
	if cmd.UpdateVerifier != nil {
		return cmd.UpdateVerifier, nil
	}
	return releaseupdate.NewEmbeddedVerifier()
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
