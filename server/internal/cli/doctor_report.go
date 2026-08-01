package cli

import (
	"context"
	"os"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"gopkg.in/yaml.v3"
)

type DoctorIssue struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation"`
}

type DoctorReport struct {
	Issues          []DoctorIssue                  `json:"issues"`
	RecoverySummary *recovery.CompatibilitySummary `json:"recovery_summary,omitempty"`
}

func validateConfigSchema(schemaPath string) error {
	if config.IsConfigUserSchemaID(schemaPath) {
		_, err := config.CompileJSON(config.ConfigUserSchemaID, config.ConfigUserSchemaJSON)
		return err
	}
	_, err := config.Compile(schemaPath)
	return err
}

func displaySchemaPath(repoRoot, schemaPath string) string {
	if schemaPath == "" {
		return config.ConfigUserSchemaID
	}
	return displayLogPath(repoRoot, schemaPath)
}

func runDoctor(cmd Command) int {
	report := BuildDoctorReport(cmd)

	hasProblems := false
	for _, issue := range report.Issues {
		if issue.Severity != "ok" {
			cmd.Logger.Warn(issue.Summary, "code", issue.Code)
			hasProblems = true
		} else {
			cmd.Logger.Info(issue.Summary, "code", issue.Code)
		}
	}

	if hasProblems {
		cmd.Logger.Warn("服务自检完成，发现需要处理的问题")
		return 1
	}
	cmd.Logger.Info("服务自检完成，所有检查通过")
	return 0
}

func BuildDoctorReport(cmd Command) DoctorReport {
	issues := make([]DoctorIssue, 0, 8)
	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
	configPathDisplay := displayLogPath(repoRoot, cmd.ConfigPath)

	if _, err := os.Stat(cmd.ConfigPath); err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "config.not_accessible",
			Severity:    "error",
			Summary:     "配置文件不可访问：" + configPathDisplay,
			Remediation: "请确认配置文件路径正确且可读。",
		})
	} else {
		issues = append(issues, DoctorIssue{
			Code:     "config.ok",
			Severity: "ok",
			Summary:  "配置文件可访问：" + configPathDisplay,
		})
		issues = append(issues, retiredPluginRuntimeConfigIssues(cmd.ConfigPath)...)
	}

	if err := validateConfigSchema(cmd.SchemaPath); err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "schema.invalid",
			Severity:    "error",
			Summary:     "配置校验规则不可用：" + displaySchemaPath(repoRoot, cmd.SchemaPath),
			Remediation: "请确认配置校验规则可用。",
		})
	} else {
		issues = append(issues, DoctorIssue{
			Code:     "schema.ok",
			Severity: "ok",
			Summary:  "配置校验规则可用：" + displaySchemaPath(repoRoot, cmd.SchemaPath),
		})
	}

	databasePath, err := resolveDatabasePath(cmd.ConfigPath)
	if err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "database.path_unresolvable",
			Severity:    "error",
			Summary:     "无法从配置文件解析数据库路径：" + configPathDisplay,
			Remediation: "请确认配置文件路径正确。",
		})
	} else {
		databasePathDisplay := displayLogPath(repoRoot, databasePath)
		if err := storage.QuickCheckPath(context.Background(), databasePath); err != nil {
			issues = append(issues, DoctorIssue{
				Code:        "database.ping_failed",
				Severity:    "error",
				Summary:     "数据库完整性检查失败：" + databasePathDisplay,
				Remediation: "数据库可能损坏。请查看 data/quarantine/ 与 data/sqlite-snapshots/。",
			})
		} else {
			issues = append(issues, DoctorIssue{
				Code:     "database.ok",
				Severity: "ok",
				Summary:  "数据库可访问：" + databasePathDisplay,
			})
		}
	}

	currentPlatform := deps.CurrentPlatform()
	if manifest, err := deps.LoadManifest(repoRoot); err != nil {
		issues = append(issues, depsManifestDoctorIssues(err)...)
	} else {
		issues = append(issues, depsManifestPlatformIssue(manifest, currentPlatform))
		issues = append(issues, chromiumMetadataIssue(manifest, currentPlatform))
	}

	report := DoctorReport{Issues: issues}
	summary, err := recovery.LoadSummary(repoRoot)
	if err == nil && summary != nil {
		report.RecoverySummary = summary
	}
	return report
}

func retiredPluginRuntimeConfigIssues(configPath string) []DoctorIssue {
	payload, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var document map[string]any
	if yaml.Unmarshal(payload, &document) != nil {
		return nil
	}
	runtimeSection, ok := document["runtime"].(map[string]any)
	if !ok {
		return nil
	}
	retired := make([]string, 0, 3)
	for _, key := range []string{
		"nodejs_max_old_space_size_mb",
		"dependency_install_timeout_seconds",
		"max_concurrent_dependency_installs",
	} {
		if _, exists := runtimeSection[key]; exists {
			retired = append(retired, "runtime."+key)
		}
	}
	if len(retired) == 0 {
		return nil
	}
	return []DoctorIssue{{
		Code:        "config.retired_plugin_runtime_keys",
		Severity:    "error",
		Summary:     "配置仍包含已退役的 Python/Node.js 插件运行时键：" + strings.Join(retired, "、"),
		Remediation: "删除这些键；Go 插件 artifact 不使用语言运行时或依赖安装配置。",
	}}
}

func depsManifestDoctorIssues(err error) []DoctorIssue {
	if os.IsNotExist(err) {
		return []DoctorIssue{{
			Code:        "deps.manifest_missing",
			Severity:    "warning",
			Summary:     "依赖清单缺失。",
			Remediation: "请恢复 .deps/manifest.json。",
		}}
	}
	return []DoctorIssue{{
		Code:        "deps.manifest_invalid",
		Severity:    "warning",
		Summary:     "依赖清单格式无效。",
		Remediation: "请重新生成 .deps/manifest.json。",
	}}
}

func depsManifestPlatformIssue(manifest *deps.Manifest, platform string) DoctorIssue {
	if manifest.HasPlatform(platform) {
		return DoctorIssue{
			Code:     "deps.manifest",
			Severity: "ok",
			Summary:  "依赖清单已包含当前平台资源。",
		}
	}
	return DoctorIssue{
		Code:        "deps.manifest_platform_missing",
		Severity:    "warning",
		Summary:     "依赖清单缺少当前平台资源。",
		Remediation: "请为当前平台重新生成或恢复 .deps/manifest.json。",
	}
}
