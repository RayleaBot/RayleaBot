package cli

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"rayleabot/server/internal/storage"
)

type Command struct {
	Name        string
	ConfigPath  string
	SchemaPath  string
	Logger      *slog.Logger
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
		fmt.Fprintln(os.Stderr, "backup 子命令尚未实现")
		return 1
	case "restore":
		fmt.Fprintln(os.Stderr, "restore 子命令尚未实现")
		return 1
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

func runDoctor(cmd Command) int {
	issues := 0

	// Check config file exists.
	if _, err := os.Stat(cmd.ConfigPath); err != nil {
		cmd.Logger.Warn("config file not accessible", "path", cmd.ConfigPath, "err", err.Error())
		issues++
	} else {
		cmd.Logger.Info("config file OK", "path", cmd.ConfigPath)
	}

	// Check schema file exists.
	if _, err := os.Stat(cmd.SchemaPath); err != nil {
		cmd.Logger.Warn("config schema file not accessible", "path", cmd.SchemaPath, "err", err.Error())
		issues++
	} else {
		cmd.Logger.Info("config schema file OK", "path", cmd.SchemaPath)
	}

	// Check database.
	databasePath, err := resolveDatabasePath(cmd.ConfigPath)
	if err != nil {
		cmd.Logger.Warn("could not resolve database path", "err", err.Error())
		issues++
	} else {
		db, err := sql.Open("sqlite", databasePath)
		if err != nil {
			cmd.Logger.Warn("database open failed", "path", databasePath, "err", err.Error())
			issues++
		} else {
			defer db.Close()
			if err := db.Ping(); err != nil {
				cmd.Logger.Warn("database ping failed", "path", databasePath, "err", err.Error())
				issues++
			} else {
				cmd.Logger.Info("database OK", "path", databasePath)
			}

			// Check schema_migrations table.
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
				cmd.Logger.Warn("schema_migrations not accessible", "err", err.Error())
				issues++
			} else {
				cmd.Logger.Info("schema migrations applied", "count", count)
			}
		}
	}

	// Check contracts directory.
	contractsDir := filepath.Dir(cmd.SchemaPath)
	entries, err := os.ReadDir(contractsDir)
	if err != nil {
		cmd.Logger.Warn("contracts directory not accessible", "path", contractsDir, "err", err.Error())
		issues++
	} else {
		cmd.Logger.Info("contracts directory OK", "path", contractsDir, "files", len(entries))
	}

	// Check Python availability.
	checkExecutable(cmd.Logger, &issues, "python3", "python")

	// Check Node.js availability.
	checkExecutable(cmd.Logger, &issues, "node")

	// Check npm availability.
	if isWindows() {
		checkExecutable(cmd.Logger, &issues, "npm.cmd", "npm")
	} else {
		checkExecutable(cmd.Logger, &issues, "npm")
	}

	if issues > 0 {
		cmd.Logger.Warn("doctor completed with issues", "issue_count", issues)
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
	// Default database path relative to config directory.
	dbPath := filepath.Join(configDir, "..", "data", "state.db")
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	return absPath, nil
}

func checkExecutable(logger *slog.Logger, issues *int, names ...string) {
	for _, name := range names {
		path, err := lookPath(name)
		if err == nil {
			logger.Info("executable found", "name", name, "path", path)
			return
		}
	}
	logger.Warn("executable not found", "candidates", strings.Join(names, ", "))
	*issues++
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}
