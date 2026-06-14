package app

import (
	"fmt"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/redact"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/schemaassets"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type appBuildState struct {
	core             appCore
	options          Options
	logStream        *logging.Stream
	taskRegistry     *tasks.Registry
	taskExecutor     *tasks.Executor
	discoverySpec    runtimepaths.PluginDiscoverySpec
	pluginValidator  *schema.Validator
	pluginCatalog    *plugincatalog.Catalog
	managementRedact func(string) string
}

func initializeAppBuild(options Options) (appBuildState, error) {
	cfg, summary, err := config.Load(options.ConfigPath, options.SchemaPath)
	if err != nil {
		return appBuildState{}, err
	}

	managementRedactor := redact.NewManagementRedactor(cfg)
	logger, logStream, logLevel, err := logging.NewWithStreamAndController(cfg.Log.Level, managementRedactor.Redact)
	if err != nil {
		return appBuildState{}, err
	}

	taskRegistry := tasks.NewRegistry()
	taskExecutor := tasks.NewExecutor(taskRegistry, 5*time.Minute)
	discoverySpec, err := runtimepaths.ResolvePluginDiscovery(runtimepaths.PluginDiscoveryOptions{
		ConfigPath:       options.ConfigPath,
		PluginRepoRoot:   options.PluginRepoRoot,
		PluginSchemaPath: options.PluginSchemaPath,
		PluginRoots:      options.PluginRoots,
	})
	if err != nil {
		return appBuildState{}, err
	}
	pluginValidator, err := compilePluginSchema(discoverySpec.PluginSchemaPath)
	if err != nil {
		return appBuildState{}, fmt.Errorf("compile plugin manifest schema %s: %w", discoverySpec.PluginSchemaPath, err)
	}
	snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
		Validator: pluginValidator,
		Roots:     discoverySpec.Roots,
		RepoRoot:  discoverySpec.RepoRoot,
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
			repoRoot:   discoverySpec.RepoRoot,
			redactText: managementRedactor.Redact,
			startedAt:  time.Now().UTC(),
		},
		options:          options,
		logStream:        logStream,
		taskRegistry:     taskRegistry,
		taskExecutor:     taskExecutor,
		discoverySpec:    discoverySpec,
		pluginValidator:  pluginValidator,
		pluginCatalog:    plugincatalog.New(snapshots),
		managementRedact: managementRedactor.Redact,
	}, nil
}

func compilePluginSchema(schemaPath string) (*schema.Validator, error) {
	if schemaassets.IsPluginInfoSchemaID(schemaPath) {
		return schema.CompileJSON(schemaassets.PluginInfoSchemaID, schemaassets.PluginInfoSchemaJSON)
	}
	return schema.Compile(schemaPath)
}
