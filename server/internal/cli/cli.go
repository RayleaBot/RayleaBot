package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"gopkg.in/yaml.v3"

	_ "modernc.org/sqlite"
)

type Command struct {
	Name             string
	ConfigPath       string
	SchemaPath       string
	Logger           *slog.Logger
	Args             []string // additional positional arguments after the subcommand name
	Stdout           io.Writer
	UpdateVerifier   *releaseupdate.Verifier
	UpdateHTTPClient *http.Client
	Now              func() time.Time
}

func Run(cmd Command) int {
	switch cmd.Name {
	case "plugin":
		return runLifecycleLocked(cmd, "开发插件同步", runPlugin)
	case "config":
		return runConfig(cmd)
	case "reset-admin":
		return runLifecycleLocked(cmd, "管理员凭据重置", runResetAdmin)
	case "doctor":
		return runDoctor(cmd)
	case "cleanup":
		return runLifecycleLocked(cmd, "运行数据清理", runCleanup)
	case "backup":
		return runLifecycleLocked(cmd, "离线备份", runBackup)
	case "restore":
		return runLifecycleLocked(cmd, "离线恢复", runRestore)
	case "version":
		return runVersion(cmd)
	case "update":
		return runUpdate(cmd)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n", cmd.Name)
		fmt.Fprintln(os.Stderr, "可用子命令: config, plugin, reset-admin, backup, restore, doctor, cleanup, version, update")
		return 1
	}
}

func commandStdout(cmd Command) io.Writer {
	if cmd.Stdout != nil {
		return cmd.Stdout
	}
	return os.Stdout
}

func displayLogPath(repoRoot, path string) string {
	return logpath.Display(repoRoot, path)
}

func displayLogError(repoRoot string, err error, paths ...string) string {
	return logpath.Error(repoRoot, err, paths...)
}

func resolveDatabasePath(cmd Command) (string, error) {
	payload, err := os.ReadFile(cmd.ConfigPath)
	if errors.Is(err, os.ErrNotExist) {
		return runtimepaths.ResolveDatabasePath(cmd.ConfigPath, "data/rayleabot.db")
	}
	if err != nil {
		return "", fmt.Errorf("read database configuration: %w", err)
	}
	var document struct {
		Database struct {
			Path string `yaml:"path"`
		} `yaml:"database"`
	}
	if err := yaml.Unmarshal(payload, &document); err != nil {
		return "", fmt.Errorf("parse database configuration: %w", err)
	}
	databasePath := strings.TrimSpace(document.Database.Path)
	if databasePath == "" {
		databasePath = "data/rayleabot.db"
	}
	return runtimepaths.ResolveDatabasePath(cmd.ConfigPath, databasePath)
}

func runConfig(cmd Command) int {
	if len(cmd.Args) != 1 {
		fmt.Fprintln(os.Stderr, "可用子命令: config init, config normalize, config validate")
		return 1
	}

	action := cmd.Args[0]
	var err error
	switch action {
	case "init":
		err = runConfigMutation(cmd, func() error {
			_, _, mutationErr := internalconfig.Init(cmd.ConfigPath, cmd.SchemaPath)
			return mutationErr
		})
	case "normalize":
		err = runConfigMutation(cmd, func() error {
			_, _, mutationErr := internalconfig.Normalize(cmd.ConfigPath, cmd.SchemaPath)
			return mutationErr
		})
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

func runConfigMutation(cmd Command, mutate func() error) (err error) {
	lock, err := acquireLifecycleLock(cmd.ConfigPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, lock.Close())
	}()
	return mutate()
}

func acquireLifecycleLock(configPath string) (*filelock.Lock, error) {
	lockPath, err := runtimepaths.ResolveConfigLifecycleLockPath(configPath)
	if err != nil {
		return nil, err
	}
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		if errors.Is(err, filelock.ErrLocked) {
			return nil, errors.New("服务生命周期锁已被占用；请在停服窗口执行该命令")
		}
		return nil, fmt.Errorf("acquire service lifecycle lock: %w", err)
	}
	return lock, nil
}

func runLifecycleLocked(cmd Command, action string, run func(Command) int) int {
	lock, err := acquireLifecycleLock(cmd.ConfigPath)
	if err != nil {
		cmd.Logger.Error(action+"失败：请确认服务已停止", "err", err.Error())
		return 1
	}
	code := run(cmd)
	if closeErr := lock.Close(); closeErr != nil {
		cmd.Logger.Error(action+"完成后释放生命周期锁失败", "err", closeErr.Error())
		return 1
	}
	return code
}

func runResetAdmin(cmd Command) int {
	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
	configPathDisplay := displayLogPath(repoRoot, cmd.ConfigPath)
	databasePath, err := resolveDatabasePath(cmd)
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
	repoRoot := recovery.RepoRootFromConfigPath(cmd.ConfigPath)
	cfg, _, err := internalconfig.Load(cmd.ConfigPath, cmd.SchemaPath)
	if err != nil {
		cmd.Logger.Error("读取清理保留策略失败", "err", displayLogError(repoRoot, err, cmd.ConfigPath))
		return 1
	}
	now := time.Now
	if cmd.Now != nil {
		now = cmd.Now
	}
	currentTime := now().UTC()
	cleaned := 0
	failed := false

	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	entries, err := os.ReadDir(installedRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".plugin-install-") {
				continue
			}
			orphanPath := filepath.Join(installedRoot, entry.Name())
			if !pathOlderThan(orphanPath, currentTime.Add(-24*time.Hour)) {
				continue
			}
			orphanPathDisplay := displayLogPath(repoRoot, orphanPath)
			if err := os.RemoveAll(orphanPath); err != nil {
				cmd.Logger.Warn("清理遗留插件安装目录失败："+orphanPathDisplay, "path", orphanPathDisplay, "err", displayLogError(repoRoot, err, orphanPath))
				failed = true
			} else {
				cmd.Logger.Info("遗留插件安装目录已清理："+orphanPathDisplay, "path", orphanPathDisplay)
				cleaned++
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		cmd.Logger.Warn("读取插件安装目录失败", "err", displayLogError(repoRoot, err, installedRoot))
		failed = true
	}

	downloadRetentionDays := cfg.Data.DownloadCacheRetentionDays
	if downloadRetentionDays <= 0 {
		downloadRetentionDays = 15
	}
	cleanedDownloads, downloadFailed := cleanupEntriesOlderThan(cmd, repoRoot, filepath.Join(repoRoot, "cache", "downloads"), currentTime.AddDate(0, 0, -downloadRetentionDays), "下载缓存")
	cleaned += cleanedDownloads
	failed = failed || downloadFailed

	cleanedRender, renderFailed := cleanupEntriesOlderThan(cmd, repoRoot, filepath.Join(repoRoot, "data", "render"), currentTime.AddDate(0, 0, -7), "渲染缓存")
	cleaned += cleanedRender
	failed = failed || renderFailed

	cmd.Logger.Info(fmt.Sprintf("清理完成，共处理 %d 项", cleaned), "cleaned_items", cleaned)
	if failed {
		return 1
	}
	return 0
}

func cleanupEntriesOlderThan(cmd Command, repoRoot, root string, cutoff time.Time, label string) (int, bool) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false
	}
	if err != nil {
		cmd.Logger.Warn("读取"+label+"目录失败", "err", displayLogError(repoRoot, err, root))
		return 0, true
	}
	cleaned := 0
	failed := false
	for _, entry := range entries {
		entryPath := filepath.Join(root, entry.Name())
		if !pathOlderThan(entryPath, cutoff) {
			continue
		}
		entryPathDisplay := displayLogPath(repoRoot, entryPath)
		if err := os.RemoveAll(entryPath); err != nil {
			cmd.Logger.Warn("清理"+label+"条目失败："+entryPathDisplay, "path", entryPathDisplay, "err", displayLogError(repoRoot, err, entryPath))
			failed = true
			continue
		}
		cleaned++
	}
	if cleaned > 0 {
		rootDisplay := displayLogPath(repoRoot, root)
		cmd.Logger.Info(fmt.Sprintf("%s已清理：%s，条目 %d 个", label, rootDisplay, cleaned), "path", rootDisplay, "entries", cleaned)
	}
	return cleaned, failed
}

func pathOlderThan(path string, cutoff time.Time) bool {
	info, err := os.Stat(path)
	return err == nil && !info.ModTime().After(cutoff)
}
