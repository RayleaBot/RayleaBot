package cli

import (
	"context"
	"os"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func BuildDoctorReport(cmd Command) DoctorReport {
	issues := make([]DoctorIssue, 0, 8)

	if _, err := os.Stat(cmd.ConfigPath); err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "config.not_accessible",
			Severity:    "error",
			Summary:     "Config file not accessible: " + cmd.ConfigPath,
			Remediation: "请确认配置文件路径正确且可读。",
		})
	} else {
		issues = append(issues, DoctorIssue{
			Code:     "config.ok",
			Severity: "ok",
			Summary:  "Config file accessible",
		})
	}

	if err := validateConfigSchema(cmd.SchemaPath); err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "schema.invalid",
			Severity:    "error",
			Summary:     "Config schema unavailable: " + displaySchemaPath(cmd.SchemaPath),
			Remediation: "请确认配置校验规则可用。",
		})
	} else {
		issues = append(issues, DoctorIssue{
			Code:     "schema.ok",
			Severity: "ok",
			Summary:  "Config schema available",
		})
	}

	databasePath, err := resolveDatabasePath(cmd.ConfigPath)
	if err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "database.path_unresolvable",
			Severity:    "error",
			Summary:     "Could not resolve database path",
			Remediation: "请确认配置文件路径正确。",
		})
	} else {
		if err := storage.QuickCheckPath(context.Background(), databasePath); err != nil {
			issues = append(issues, DoctorIssue{
				Code:        "database.ping_failed",
				Severity:    "error",
				Summary:     "Database integrity check failed: " + databasePath,
				Remediation: "数据库可能损坏。请查看 data/quarantine/ 与 data/sqlite-snapshots/。",
			})
		} else {
			issues = append(issues, DoctorIssue{
				Code:     "database.ok",
				Severity: "ok",
				Summary:  "Database accessible",
			})
		}
	}

	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
	currentPlatform := currentManifestPlatform()
	if manifest, err := loadDepsManifest(repoRoot); err != nil {
		issues = append(issues, depsManifestDoctorIssues(err)...)
	} else {
		issues = append(issues, depsManifestPlatformIssue(manifest, currentPlatform))
		issues = append(issues, runtimeMetadataIssue(manifest, currentPlatform, "python-runtime", "Python 运行环境", "deps.python_runtime_metadata", "deps.python_runtime_metadata_incomplete"))
		issues = append(issues, runtimeMetadataIssue(manifest, currentPlatform, "nodejs-runtime", "Node.js / npm 环境", "deps.nodejs_runtime_metadata", "deps.nodejs_runtime_metadata_incomplete"))
		issues = append(issues, managedRuntimeDoctorIssues(repoRoot)...)
	}

	report := DoctorReport{Issues: issues}
	summary, err := recovery.LoadSummary(repoRoot)
	if err == nil && summary != nil {
		report.RecoverySummary = summary
	}
	return report
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

func depsManifestPlatformIssue(manifest *depsManifest, platform string) DoctorIssue {
	if manifest.hasPlatform(platform) {
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
