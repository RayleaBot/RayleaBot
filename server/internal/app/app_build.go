package app

import (
	"context"
	"fmt"
	"time"

	"rayleabot/server/internal/config"
	"rayleabot/server/internal/deps"
	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/schema"
	"rayleabot/server/internal/tasks"
)

var resolveManagedRenderBrowserPath = func(ctx context.Context, repoRoot string) (string, error) {
	return deps.NewManager(repoRoot).ResolveEntrypoint(ctx, "chromium", "browser")
}

type appBuildState struct {
	core             appCore
	options          Options
	logStream        *logging.Stream
	taskRegistry     *tasks.Registry
	taskExecutor     *tasks.Executor
	discoverySpec    pluginDiscoverySpec
	pluginValidator  *schema.Validator
	pluginCatalog    *plugins.Catalog
	managementRedact func(string) string
}

func initializeAppBuild(options Options) (appBuildState, error) {
	cfg, summary, err := config.Load(options.ConfigPath, options.SchemaPath)
	if err != nil {
		return appBuildState{}, err
	}

	managementRedactor := buildManagementRedactor(cfg)
	logger, logStream, logLevel, err := logging.NewWithStreamAndController(cfg.Logging.Level, managementRedactor.Redact)
	if err != nil {
		return appBuildState{}, err
	}

	taskRegistry := tasks.NewRegistry()
	taskExecutor := tasks.NewExecutor(taskRegistry, 5*time.Minute)
	discoverySpec, err := resolvePluginDiscovery(options)
	if err != nil {
		return appBuildState{}, err
	}
	pluginValidator, err := schema.Compile(discoverySpec.pluginSchemaPath)
	if err != nil {
		return appBuildState{}, fmt.Errorf("compile plugin manifest schema %s: %w", discoverySpec.pluginSchemaPath, err)
	}
	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: pluginValidator,
		Roots:     discoverySpec.roots,
		RepoRoot:  discoverySpec.repoRoot,
		Logger:    logger,
	})
	if err != nil {
		return appBuildState{}, err
	}

	return appBuildState{
		core: appCore{
			Config:     cfg,
			Summary:    summary,
			Logger:     logger,
			LogLevel:   logLevel,
			repoRoot:   discoverySpec.repoRoot,
			redactText: managementRedactor.Redact,
			startedAt:  time.Now().UTC(),
		},
		options:          options,
		logStream:        logStream,
		taskRegistry:     taskRegistry,
		taskExecutor:     taskExecutor,
		discoverySpec:    discoverySpec,
		pluginValidator:  pluginValidator,
		pluginCatalog:    plugins.NewCatalog(snapshots),
		managementRedact: managementRedactor.Redact,
	}, nil
}
