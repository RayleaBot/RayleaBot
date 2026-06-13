package cli

import (
	"fmt"
	"log/slog"
	"os"
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
