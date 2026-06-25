package main

import (
	"flag"
	"os"

	"github.com/RayleaBot/RayleaBot/server/internal/bootstrap"
	"github.com/RayleaBot/RayleaBot/server/internal/cli"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
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
	bootstrapLogger.Info(
		"starting raylea-server shell",
		"component", "main",
		"config_path", configPath,
		"schema_path", schemaPath,
	)

	runCtx, stop := bootstrap.SignalContext()
	defer stop()

	application, err := bootstrap.NewWithContext(runCtx, bootstrap.Options{
		ConfigPath: configPath,
		SchemaPath: schemaPath,
	})
	if err != nil {
		bootstrapLogger.Error(
			"startup failed",
			"component", "main",
			"config_path", configPath,
			"schema_path", schemaPath,
			"err", err.Error(),
		)
		os.Exit(1)
	}

	if err := application.Run(runCtx); err != nil {
		application.Logger().Error("server exited with error", "component", "main", "err", err.Error())
		os.Exit(1)
	}
}
