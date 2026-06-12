package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/schemaassets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
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
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n", cmd.Name)
		fmt.Fprintln(os.Stderr, "可用子命令: reset-admin, backup, restore, doctor, cleanup")
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

	// Check config schema.
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
		issues = append(issues, runtimeMetadataIssue(manifest, currentPlatform, "python-runtime", "Python 运行环境", "deps.python_runtime_metadata", "deps.python_runtime_metadata_incomplete"))
		issues = append(issues, runtimeMetadataIssue(manifest, currentPlatform, "nodejs-runtime", "Node.js / npm 环境", "deps.nodejs_runtime_metadata", "deps.nodejs_runtime_metadata_incomplete"))
		issues = append(issues, managedRuntimeDoctorIssue(
			repoRoot,
			"python-runtime",
			"runtime.python_managed_ready",
			"Python 运行环境已准备完成。",
			"Python 运行环境已下载，启动时会解压。",
			"Python 运行环境已纳入启动流程。",
			"Python 运行环境当前不可准备。",
			"请在 .deps/manifest.json 中补齐当前平台 Python 运行环境的 archive_format、entrypoints、来源列表与 sha256。",
		))
		issues = append(issues, managedRuntimeDoctorIssue(
			repoRoot,
			"nodejs-runtime",
			"runtime.node_managed_ready",
			"Node.js / npm 环境已准备完成。",
			"Node.js / npm 环境已下载，启动时会解压。",
			"Node.js / npm 环境已纳入启动流程。",
			"Node.js / npm 环境当前不可准备。",
			"请在 .deps/manifest.json 中补齐当前平台 Node.js / npm 环境的 archive_format、entrypoints、来源列表与 sha256。",
		))
		issues = append(issues, managedRuntimeDoctorIssue(
			repoRoot,
			"nodejs-runtime",
			"runtime.npm_managed_ready",
			"npm 已准备完成。",
			"npm 已下载，启动时会解压。",
			"npm 已纳入启动流程。",
			"npm 当前不可准备。",
			"请在 .deps/manifest.json 中补齐当前平台 Node.js / npm 环境的 archive_format、entrypoints、来源列表与 sha256。",
		))
	}

	report := DoctorReport{Issues: issues}
	summary, err := recovery.LoadSummary(repoRoot)
	if err == nil && summary != nil {
		report.RecoverySummary = summary
	}
	return report
}

func validateConfigSchema(schemaPath string) error {
	if schemaassets.IsConfigUserSchemaID(schemaPath) {
		_, err := schema.CompileJSON(schemaassets.ConfigUserSchemaID, schemaassets.ConfigUserSchemaJSON)
		return err
	}
	_, err := schema.Compile(schemaPath)
	return err
}

func displaySchemaPath(schemaPath string) string {
	if schemaPath == "" {
		return schemaassets.ConfigUserSchemaID
	}
	return schemaPath
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
		Remediation: "请在 .deps/manifest.json 中补齐当前平台 " + label + " 的来源列表、archive_format、entrypoints 与 sha256。",
	}
}

func managedRuntimeDoctorIssue(
	repoRoot string,
	kind string,
	code string,
	readySummary string,
	cachedSummary string,
	onDemandSummary string,
	warningSummary string,
	metadataRemediation string,
) DoctorIssue {
	inspection, err := deps.NewManager(repoRoot).Inspect(kind)
	if err != nil {
		var bootstrapErr *deps.BootstrapError
		if errors.As(err, &bootstrapErr) {
			return DoctorIssue{
				Code:        code,
				Severity:    "warning",
				Summary:     warningSummary,
				Remediation: bootstrapErr.Remediation,
			}
		}
		return DoctorIssue{
			Code:        code,
			Severity:    "warning",
			Summary:     warningSummary,
			Remediation: metadataRemediation,
		}
	}
	if !inspection.MetadataComplete {
		return DoctorIssue{
			Code:        code,
			Severity:    "warning",
			Summary:     warningSummary,
			Remediation: metadataRemediation,
		}
	}
	switch {
	case inspection.PreparedStorePresent:
		return DoctorIssue{Code: code, Severity: "ok", Summary: readySummary}
	case inspection.CachedArchivePresent:
		return DoctorIssue{Code: code, Severity: "ok", Summary: cachedSummary}
	default:
		return DoctorIssue{Code: code, Severity: "ok", Summary: onDemandSummary}
	}
}
