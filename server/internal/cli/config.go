package cli

import (
	"fmt"
	"os"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

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
	if err != nil {
		cmd.Logger.Error("config "+action+" failed", "config_path", cmd.ConfigPath, "err", err.Error())
		return 1
	}
	cmd.Logger.Info("config "+action+" completed", "config_path", cmd.ConfigPath)
	return 0
}
