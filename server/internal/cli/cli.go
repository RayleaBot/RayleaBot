package cli

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"

	_ "modernc.org/sqlite"
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
	case "config":
		return runConfig(cmd)
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
		fmt.Fprintln(os.Stderr, "可用子命令: config, reset-admin, backup, restore, doctor, cleanup")
		return 1
	}
}

func displayLogPath(repoRoot, path string) string {
	return logpath.Display(repoRoot, path)
}

func displayLogError(repoRoot string, err error, paths ...string) string {
	return logpath.Error(repoRoot, err, paths...)
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

func runConfig(cmd Command) int {
	if len(cmd.Args) == 0 {
		fmt.Fprintln(os.Stderr, "可用子命令: config init, config normalize, config validate")
		return 1
	}

	action := cmd.Args[0]
	var err error
	switch action {
	case "init":
		_, _, err = internalconfig.Init(cmd.ConfigPath, cmd.SchemaPath)
	case "normalize":
		_, _, err = internalconfig.Normalize(cmd.ConfigPath, cmd.SchemaPath)
	case "validate":
		_, _, err = internalconfig.Validate(cmd.ConfigPath, cmd.SchemaPath)
	default:
		fmt.Fprintf(os.Stderr, "未知配置子命令: %s\n", action)
		fmt.Fprintln(os.Stderr, "可用子命令: config init, config normalize, config validate")
		return 1
	}
	actionLabel := configActionLabel(action)
	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
	configPathDisplay := displayLogPath(repoRoot, cmd.ConfigPath)
	if err != nil {
		cmd.Logger.Error("配置文件"+actionLabel+"失败："+configPathDisplay, "config_path", configPathDisplay, "action", action, "err", displayLogError(repoRoot, err, cmd.ConfigPath))
		return 1
	}
	cmd.Logger.Info("配置文件"+actionLabel+"完成："+configPathDisplay, "config_path", configPathDisplay, "action", action)
	return 0
}

func configActionLabel(action string) string {
	switch action {
	case "init":
		return "初始化"
	case "normalize":
		return "规范化"
	case "validate":
		return "校验"
	default:
		return action
	}
}

func runResetAdmin(cmd Command) int {
	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
	configPathDisplay := displayLogPath(repoRoot, cmd.ConfigPath)
	databasePath, err := resolveDatabasePath(cmd.ConfigPath)
	if err != nil {
		cmd.Logger.Error("解析数据库路径失败："+configPathDisplay, "config_path", configPathDisplay, "err", displayLogError(repoRoot, err, cmd.ConfigPath))
		return 1
	}
	databasePathDisplay := displayLogPath(repoRoot, databasePath)

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		cmd.Logger.Error("打开数据库失败："+databasePathDisplay, "path", databasePathDisplay, "err", displayLogError(repoRoot, err, databasePath))
		return 1
	}
	defer db.Close()

	tables := []string{"admin_sessions", "auth_bootstrap_state"}
	for _, table := range tables {
		if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			cmd.Logger.Error("清空管理员状态表失败："+table, "table", table, "err", err.Error())
			return 1
		}
		cmd.Logger.Info("管理员状态表已清空："+table, "table", table)
	}

	cmd.Logger.Info("管理员凭据已重置，下次启动将进入初始设置状态："+databasePathDisplay, "path", databasePathDisplay)
	return 0
}

func runCleanup(cmd Command) int {
	configDir := filepath.Dir(cmd.ConfigPath)
	repoRoot := filepath.Dir(configDir)
	cleaned := 0

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
				orphanPathDisplay := displayLogPath(repoRoot, orphanPath)
				if err := os.RemoveAll(orphanPath); err != nil {
					cmd.Logger.Warn("清理遗留插件安装目录失败："+orphanPathDisplay, "path", orphanPathDisplay, "err", displayLogError(repoRoot, err, orphanPath))
				} else {
					cmd.Logger.Info("遗留插件安装目录已清理："+orphanPathDisplay, "path", orphanPathDisplay)
					cleaned++
				}
			}
		}
	}

	cacheRoot := filepath.Join(repoRoot, "cache", "downloads")
	if _, err := os.Stat(cacheRoot); err == nil {
		cacheEntries, err := os.ReadDir(cacheRoot)
		if err == nil {
			for _, entry := range cacheEntries {
				entryPath := filepath.Join(cacheRoot, entry.Name())
				entryPathDisplay := displayLogPath(repoRoot, entryPath)
				if err := os.RemoveAll(entryPath); err != nil {
					cmd.Logger.Warn("清理下载缓存条目失败："+entryPathDisplay, "path", entryPathDisplay, "err", displayLogError(repoRoot, err, entryPath))
				} else {
					cleaned++
				}
			}
			if len(cacheEntries) > 0 {
				cacheRootDisplay := displayLogPath(repoRoot, cacheRoot)
				cmd.Logger.Info(fmt.Sprintf("下载缓存已清理：%s，条目 %d 个", cacheRootDisplay, len(cacheEntries)), "path", cacheRootDisplay, "entries", len(cacheEntries))
			}
		}
	}

	cmd.Logger.Info(fmt.Sprintf("清理完成，共处理 %d 项", cleaned), "cleaned_items", cleaned)
	return 0
}
