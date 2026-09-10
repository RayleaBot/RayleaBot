package app

import (
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/redact"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type appBuildState struct {
	core             *appRuntimeState
	options          Options
	logStream        *logging.Stream
	taskRegistry     *tasks.Registry
	taskExecutor     *tasks.Executor
	discoverySpec    plugincatalog.DiscoverySpec
	pluginValidator  *config.Validator
	pluginCatalog    *plugincatalog.Catalog
	managementRedact func(string) string
}

func initializeAppBuild(options Options) (appBuildState, error) {
	cfg, summary, err := config.Load(options.ConfigPath, options.SchemaPath)
	if err != nil {
		return appBuildState{}, err
	}

	managementRedactor := redact.NewManagementRedactor(cfg)
	location, err := config.LoadTimezone(cfg.Scheduler.Timezone)
	if err != nil {
		return appBuildState{}, err
	}
	logger, logStream, logLevel, err := logging.NewWithStreamAndController(cfg.Log.Level, managementRedactor.Redact, location)
	if err != nil {
		return appBuildState{}, err
	}

	discoverySpec, err := plugincatalog.ResolveDiscovery(plugincatalog.DiscoveryOptions{
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
	snapshots, _, err := plugincatalog.Discover(plugincatalog.DiscoverOptions{
		Validator: pluginValidator,
		Roots:     discoverySpec.Roots,
		RepoRoot:  discoverySpec.RepoRoot,
		Logger:    logger,
	})
	if err != nil {
		return appBuildState{}, err
	}

	core := &appRuntimeState{
		Logger:             logger,
		LogLevel:           logLevel,
		repoRoot:           discoverySpec.RepoRoot,
		redactText:         managementRedactor.Redact,
		addRedactionValues: managementRedactor.Add,
		startedAt:          time.Now().UTC(),
	}
	core.SetConfig(cfg)
	core.SetSummary(summary)
	taskRegistry := tasks.NewRegistry()
	taskExecutor := tasks.NewExecutor(taskRegistry, 5*time.Minute)

	return appBuildState{
		core:             core,
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

func compilePluginSchema(schemaPath string) (*config.Validator, error) {
	if config.IsPluginInfoSchemaID(schemaPath) {
		return config.CompileJSON(config.PluginInfoSchemaID, config.PluginInfoSchemaJSON)
	}
	return config.Compile(schemaPath)
}
