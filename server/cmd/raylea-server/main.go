package main

import (
	"flag"
	"os"

	"github.com/RayleaBot/RayleaBot/server/internal/bootstrap"
	"github.com/RayleaBot/RayleaBot/server/internal/cli"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/schemaassets"
)

func main() {
	var configPath string
	var schemaPath string

	flag.StringVar(&configPath, "config", "config/user.yaml", "path to config/user.yaml")
	flag.StringVar(&schemaPath, "config-schema", schemaassets.ConfigUserSchemaID, "path to config.user.schema.json or builtin schema id")
	flag.Parse()

	// If a subcommand is provided as the first non-flag argument, dispatch to CLI.
	args := flag.Args()
	if len(args) > 0 {
		logger := logging.Bootstrap()
		exitCode := cli.Run(cli.Command{
			Name:       args[0],
			ConfigPath: configPath,
			SchemaPath: schemaPath,
			Logger:     logger,
			Args:       args[1:],
		})
		os.Exit(exitCode)
	}

	bootstrapLogger := logging.Bootstrap()
	repoRoot := recovery.RepoRootFromConfigPath(configPath)
	configPathDisplay := logpath.Display(repoRoot, configPath)
	schemaPathDisplay := logpath.Display(repoRoot, schemaPath)
	bootstrapLogger.Info(
		"RayleaBot 服务进程正在启动，配置文件："+configPathDisplay,
		"component", "main",
		"config_path", configPathDisplay,
		"schema_path", schemaPathDisplay,
	)

	runCtx, stop := bootstrap.SignalContext()
	defer stop()

	application, err := bootstrap.NewWithContext(runCtx, bootstrap.Options{
		ConfigPath: configPath,
		SchemaPath: schemaPath,
	})
	if err != nil {
		bootstrapLogger.Error(
			"RayleaBot 服务启动失败，配置文件："+configPathDisplay,
			"component", "main",
			"config_path", configPathDisplay,
			"schema_path", schemaPathDisplay,
			"err", logpath.Error(repoRoot, err, configPath, schemaPath),
		)
		os.Exit(1)
	}

	if err := application.Run(runCtx); err != nil {
		application.Logger().Error("RayleaBot 服务运行异常退出", "component", "main", "err", err.Error())
		os.Exit(1)
	}
}
