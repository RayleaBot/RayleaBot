package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/cli"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func main() {
	setupTokenFromEnv := consumeSecretEnv("RAYLEA_SETUP_TOKEN")
	launcherControlToken := consumeSecretEnv("RAYLEA_LAUNCHER_CONTROL_TOKEN")
	var configPath string
	var schemaPath string

	flag.StringVar(&configPath, "config", "config/user.yaml", "path to config/user.yaml")
	flag.StringVar(&schemaPath, "config-schema", config.ConfigUserSchemaID, "path to config.user.schema.json or builtin schema id")
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

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	independentSetupToken := setupTokenFromEnv == ""
	if independentSetupToken {
		generatedSetupToken, tokenErr := auth.GenerateOpaqueToken(32)
		if tokenErr != nil {
			bootstrapLogger.Error("生成首次设置凭据失败", "component", "main", "err", tokenErr.Error())
			os.Exit(1)
		}
		setupTokenFromEnv = generatedSetupToken
	}

	application, err := app.NewWithContext(runCtx, app.Options{
		ConfigPath:           configPath,
		SchemaPath:           schemaPath,
		SetupToken:           setupTokenFromEnv,
		LauncherControlToken: launcherControlToken,
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
	if independentSetupToken && isInteractiveConsole() {
		_, _ = fmt.Fprintf(os.Stderr, "首次设置地址（仅显示一次）：%s\n", setupURL(application.CurrentConfig(), setupTokenFromEnv))
	}

	if err := application.Run(runCtx); err != nil {
		application.Logger().Error(
			"RayleaBot 服务运行异常退出",
			"component", "main",
			"error_code", "platform.internal_error",
			"err", err.Error(),
		)
		os.Exit(1)
	}
}

func consumeSecretEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	_ = os.Unsetenv(name)
	return value
}

func isInteractiveConsole() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func setupURL(cfg config.Config, setupToken string) string {
	host := strings.TrimSpace(cfg.Server.Host)
	if host == "0.0.0.0" || host == "::" || host == "" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(cfg.Server.Port)) +
		"/setup#setup_token=" + url.QueryEscape(setupToken)
}
