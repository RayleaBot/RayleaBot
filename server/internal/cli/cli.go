package cli

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"rayleabot/server/internal/recovery"
	"rayleabot/server/internal/storage"
)

type Command struct {
	Name       string
	ConfigPath string
	SchemaPath string
	Logger     *slog.Logger
	Args       []string // additional positional arguments after the subcommand name
}

func Run(cmd Command) int {
	switch cmd.Name {
	case "reset-admin":
		return runResetAdmin(cmd)
	case "doctor":
		return runDoctor(cmd)
	case "cleanup":
		return runCleanup(cmd)
	case "backup":
		return runBackup(cmd)
	case "restore":
		return runRestore(cmd)
	case "migrate":
		return runMigrate(cmd)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n", cmd.Name)
		fmt.Fprintln(os.Stderr, "可用子命令: reset-admin, backup, restore, doctor, migrate, cleanup")
		return 1
	}
}

func runResetAdmin(cmd Command) int {
	databasePath, err := resolveDatabasePath(cmd.ConfigPath)
	if err != nil {
		cmd.Logger.Error("resolve database path", "err", err.Error())
		return 1
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		cmd.Logger.Error("open database", "path", databasePath, "err", err.Error())
		return 1
	}
	defer db.Close()

	tables := []string{"admin_sessions", "auth_bootstrap_state"}
	for _, table := range tables {
		if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			cmd.Logger.Error("clear table", "table", table, "err", err.Error())
			return 1
		}
		cmd.Logger.Info("cleared table", "table", table)
	}

	cmd.Logger.Info("admin credentials reset; server will enter setup_required on next start")
	return 0
}

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

func BuildDoctorReport(cmd Command) DoctorReport {
	issues := make([]DoctorIssue, 0, 8)

	// Check config file.
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

	// Check schema file.
	if _, err := os.Stat(cmd.SchemaPath); err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "schema.not_accessible",
			Severity:    "error",
			Summary:     "Config schema file not accessible: " + cmd.SchemaPath,
			Remediation: "请确认 contracts 目录完整。",
		})
	} else {
		issues = append(issues, DoctorIssue{
			Code:     "schema.ok",
			Severity: "ok",
			Summary:  "Config schema file accessible",
		})
	}

	// Check database.
	databasePath, err := resolveDatabasePath(cmd.ConfigPath)
	if err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "database.path_unresolvable",
			Severity:    "error",
			Summary:     "Could not resolve database path",
			Remediation: "请确认配置文件路径正确。",
		})
	} else {
		db, err := sql.Open("sqlite", databasePath)
		if err != nil {
			issues = append(issues, DoctorIssue{
				Code:        "database.open_failed",
				Severity:    "error",
				Summary:     "Database open failed: " + databasePath,
				Remediation: "请确认数据库文件未损坏且路径可访问。",
			})
		} else {
			defer db.Close()
			if err := db.Ping(); err != nil {
				issues = append(issues, DoctorIssue{
					Code:        "database.ping_failed",
					Severity:    "error",
					Summary:     "Database ping failed: " + databasePath,
					Remediation: "请确认数据库文件未损坏。",
				})
			} else {
				issues = append(issues, DoctorIssue{
					Code:     "database.ok",
					Severity: "ok",
					Summary:  "Database accessible",
				})
			}
		}
	}

	// Check contracts directory.
	contractsDir := filepath.Dir(cmd.SchemaPath)
	if _, err := os.ReadDir(contractsDir); err != nil {
		issues = append(issues, DoctorIssue{
			Code:        "contracts.not_accessible",
			Severity:    "warning",
			Summary:     "Contracts directory not accessible: " + contractsDir,
			Remediation: "请确认 contracts 目录存在且可读。",
		})
	} else {
		issues = append(issues, DoctorIssue{
			Code:     "contracts.ok",
			Severity: "ok",
			Summary:  "Contracts directory accessible",
		})
	}

	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
	currentPlatform := currentManifestPlatform()
	if manifest, err := loadDepsManifest(repoRoot); err != nil {
		if os.IsNotExist(err) {
			issues = append(issues, DoctorIssue{
				Code:        "deps.manifest_missing",
				Severity:    "warning",
				Summary:     "依赖清单缺失。",
				Remediation: "请恢复 .deps/manifest.json。",
			})
		} else {
			issues = append(issues, DoctorIssue{
				Code:        "deps.manifest_invalid",
				Severity:    "warning",
				Summary:     "依赖清单格式无效。",
				Remediation: "请重新生成 .deps/manifest.json。",
			})
		}
	} else {
		if manifest.hasPlatform(currentPlatform) {
			issues = append(issues, DoctorIssue{
				Code:     "deps.manifest",
				Severity: "ok",
				Summary:  "依赖清单已包含当前平台资源。",
			})
		} else {
			issues = append(issues, DoctorIssue{
				Code:        "deps.manifest_platform_missing",
				Severity:    "warning",
				Summary:     "依赖清单缺少当前平台资源。",
				Remediation: "请为当前平台重新生成或恢复 .deps/manifest.json。",
			})
		}
		issues = append(issues, runtimeMetadataIssue(manifest, currentPlatform, "python-runtime", "Python 运行时", "deps.python_runtime_metadata", "deps.python_runtime_metadata_incomplete"))
		issues = append(issues, runtimeMetadataIssue(manifest, currentPlatform, "nodejs-runtime", "Node.js 运行时", "deps.nodejs_runtime_metadata", "deps.nodejs_runtime_metadata_incomplete"))
		issues = append(issues, DoctorIssue{
			Code:        "runtime.python_managed_ready",
			Severity:    managedRuntimeSeverity(manifest, currentPlatform, "python-runtime"),
			Summary:     managedRuntimeSummary(manifest, currentPlatform, "python-runtime", "受控 Python 运行时可按需准备。", "受控 Python 运行时当前不可准备。"),
			Remediation: managedRuntimeRemediation(manifest, currentPlatform, "python-runtime", "请在 .deps/manifest.json 中补齐当前平台 Python 运行时的 archive_format、entrypoints、source 与 sha256。"),
		})
		issues = append(issues, DoctorIssue{
			Code:        "runtime.node_managed_ready",
			Severity:    managedRuntimeSeverity(manifest, currentPlatform, "nodejs-runtime"),
			Summary:     managedRuntimeSummary(manifest, currentPlatform, "nodejs-runtime", "受控 Node.js 运行时可按需准备。", "受控 Node.js 运行时当前不可准备。"),
			Remediation: managedRuntimeRemediation(manifest, currentPlatform, "nodejs-runtime", "请在 .deps/manifest.json 中补齐当前平台 Node.js 运行时的 archive_format、entrypoints、source 与 sha256。"),
		})
		issues = append(issues, DoctorIssue{
			Code:        "runtime.npm_managed_ready",
			Severity:    managedRuntimeSeverity(manifest, currentPlatform, "nodejs-runtime"),
			Summary:     managedRuntimeSummary(manifest, currentPlatform, "nodejs-runtime", "受控 npm 可按需准备。", "受控 npm 当前不可准备。"),
			Remediation: managedRuntimeRemediation(manifest, currentPlatform, "nodejs-runtime", "请在 .deps/manifest.json 中补齐当前平台 Node.js 运行时的 archive_format、entrypoints、source 与 sha256。"),
		})
	}

	report := DoctorReport{Issues: issues}
	summary, err := recovery.LoadSummary(repoRoot)
	if err == nil && summary != nil {
		report.RecoverySummary = summary
	}
	return report
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
		cmd.Logger.Warn("doctor completed with issues")
		return 1
	}
	cmd.Logger.Info("doctor completed, all checks passed")
	return 0
}

func runCleanup(cmd Command) int {
	configDir := filepath.Dir(cmd.ConfigPath)
	repoRoot := filepath.Dir(configDir)
	cleaned := 0

	// Clean orphaned install temp directories.
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	entries, err := os.ReadDir(installedRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if len(name) > len(".plugin-install-") && name[:len(".plugin-install-")] == ".plugin-install-" {
				orphanPath := filepath.Join(installedRoot, name)
				if err := os.RemoveAll(orphanPath); err != nil {
					cmd.Logger.Warn("failed to remove orphaned install dir", "path", orphanPath, "err", err.Error())
				} else {
					cmd.Logger.Info("removed orphaned install directory", "path", orphanPath)
					cleaned++
				}
			}
		}
	}

	// Clean download cache.
	cacheRoot := filepath.Join(repoRoot, "cache", "downloads")
	if _, err := os.Stat(cacheRoot); err == nil {
		cacheEntries, err := os.ReadDir(cacheRoot)
		if err == nil {
			for _, entry := range cacheEntries {
				entryPath := filepath.Join(cacheRoot, entry.Name())
				if err := os.RemoveAll(entryPath); err != nil {
					cmd.Logger.Warn("failed to remove cache entry", "path", entryPath, "err", err.Error())
				} else {
					cleaned++
				}
			}
			if len(cacheEntries) > 0 {
				cmd.Logger.Info("cleared download cache", "entries", len(cacheEntries))
			}
		}
	}

	cmd.Logger.Info("cleanup completed", "cleaned_items", cleaned)
	return 0
}

func runMigrate(cmd Command) int {
	databasePath, err := resolveDatabasePath(cmd.ConfigPath)
	if err != nil {
		cmd.Logger.Error("resolve database path", "err", err.Error())
		return 1
	}

	store, err := storage.Open(databasePath)
	if err != nil {
		cmd.Logger.Error("open database for migration", "path", databasePath, "err", err.Error())
		return 1
	}
	defer store.Close()

	cmd.Logger.Info("database migrations applied successfully", "path", databasePath)
	return 0
}

func resolveDatabasePath(configPath string) (string, error) {
	configDir := filepath.Dir(configPath)
	dbPath := filepath.Join(configDir, "..", "data", "rayleabot.db")
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	return absPath, nil
}

func runtimeMetadataIssue(
	manifest *depsManifest,
	platform string,
	kind string,
	label string,
	okCode string,
	incompleteCode string,
) DoctorIssue {
	resource := manifest.findResource(platform, kind)
	if manifestResourceMetadataComplete(resource) {
		return DoctorIssue{
			Code:     okCode,
			Severity: "ok",
			Summary:  label + "元数据完整。",
		}
	}
	return DoctorIssue{
		Code:        incompleteCode,
		Severity:    "warning",
		Summary:     label + "元数据不完整。",
		Remediation: "请在 .deps/manifest.json 中补齐当前平台 " + label + " 的 source 与 sha256。",
	}
}

func managedRuntimeSeverity(manifest *depsManifest, platform string, kind string) string {
	if manifestResourceMetadataComplete(manifest.findResource(platform, kind)) {
		return "ok"
	}
	return "warning"
}

func managedRuntimeSummary(manifest *depsManifest, platform string, kind string, okSummary string, warningSummary string) string {
	if manifestResourceMetadataComplete(manifest.findResource(platform, kind)) {
		return okSummary
	}
	return warningSummary
}

func managedRuntimeRemediation(manifest *depsManifest, platform string, kind string, remediation string) string {
	if manifestResourceMetadataComplete(manifest.findResource(platform, kind)) {
		return ""
	}
	return remediation
}
