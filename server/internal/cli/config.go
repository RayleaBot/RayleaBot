package cli

import (
	"fmt"
	"os"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
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
