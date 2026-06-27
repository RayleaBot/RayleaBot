package cli

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

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
