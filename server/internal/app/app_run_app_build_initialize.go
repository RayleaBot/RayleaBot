package app

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/redact"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
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
	logger, logStream, logLevel, err := logging.NewWithStreamAndController(cfg.Log.Level, managementRedactor.Redact)
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
	ensurePluginInstallRoot(logger, discoverySpec)
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

// Release packages ship without plugins/, so create the install root up front;
// otherwise a fresh installation reports the plugin directory as missing until
// the first plugin is installed.
func ensurePluginInstallRoot(logger *slog.Logger, spec plugincatalog.DiscoverySpec) {
	for _, root := range spec.Roots {
		if root.Label != "plugins/installed" {
			continue
		}
		if err := os.MkdirAll(root.Path, 0o755); err != nil {
			logger.Warn("插件安装目录创建失败", "path", logpath.Display(spec.RepoRoot, root.Path), "err", logpath.Error(spec.RepoRoot, err, root.Path))
		}
		return
	}
}
