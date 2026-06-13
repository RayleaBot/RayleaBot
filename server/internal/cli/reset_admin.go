package cli

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

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
