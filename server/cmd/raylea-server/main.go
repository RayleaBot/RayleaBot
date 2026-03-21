package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"rayleabot/server/internal/app"
	"rayleabot/server/internal/cli"
	"rayleabot/server/internal/logging"
)

func main() {
	var configPath string
	var schemaPath string

	flag.StringVar(&configPath, "config", "config/user.yaml", "path to config/user.yaml")
	flag.StringVar(&schemaPath, "config-schema", "contracts/config.user.schema.json", "path to contracts/config.user.schema.json")
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

	application, err := app.New(app.Options{
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil {
		application.Logger.Error("server exited with error", "component", "main", "err", err.Error())
		os.Exit(1)
	}
}
