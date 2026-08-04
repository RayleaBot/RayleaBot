package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func runPlugin(cmd Command) int {
	if len(cmd.Args) == 0 || cmd.Args[0] != "dev-sync" {
		fmt.Fprintln(os.Stderr, "可用子命令: plugin dev-sync")
		return 1
	}
	flags := flag.NewFlagSet("plugin dev-sync", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	artifactPath := flags.String("artifact", "", "expanded plugin artifact directory")
	sourcePath := flags.String("source", "", "plugin source repository path")
	pluginID := flags.String("plugin-id", "", "expected plugin id")
	if err := flags.Parse(cmd.Args[1:]); err != nil || *artifactPath == "" || *sourcePath == "" || *pluginID == "" {
		fmt.Fprintln(os.Stderr, "用法: raylea-server plugin dev-sync --artifact <path> --source <path> --plugin-id <id>")
		return 1
	}
	if err := syncDevelopmentPlugin(cmd, *artifactPath, *sourcePath, *pluginID); err != nil {
		cmd.Logger.Error("开发插件同步失败", "plugin_id", *pluginID, "err", err.Error())
		return 1
	}
	fmt.Fprintf(commandStdout(cmd), "已同步开发插件 %s\n", *pluginID)
	return 0
}

func syncDevelopmentPlugin(cmd Command, artifactPath, sourcePath, expectedPluginID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	repoRoot, err := runtimepaths.ResolveRuntimeRoot(cmd.ConfigPath)
	if err != nil {
		return err
	}
	artifactPath, err = filepath.Abs(artifactPath)
	if err != nil {
		return fmt.Errorf("resolve artifact path: %w", err)
	}
	sourcePath, err = filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	validator, err := internalconfig.CompileJSON(internalconfig.PluginInfoSchemaID, internalconfig.PluginInfoSchemaJSON)
	if err != nil {
		return fmt.Errorf("compile plugin schema: %w", err)
	}
	discovery := []plugincatalog.ScanRoot{{Label: "plugins/installed", Path: filepath.Join(repoRoot, "plugins", "installed")}}
	snapshots, _, err := plugincatalog.Discover(plugincatalog.DiscoverOptions{
		Validator: validator,
		Roots:     discovery,
		RepoRoot:  repoRoot,
		Logger:    cmd.Logger,
	})
	if err != nil {
		return fmt.Errorf("discover installed plugins: %w", err)
	}
	loadedConfig, _, err := internalconfig.Load(cmd.ConfigPath, cmd.SchemaPath)
	if err != nil {
		return fmt.Errorf("load development config: %w", err)
	}
	databasePath, err := runtimepaths.ResolveDatabasePath(cmd.ConfigPath, loadedConfig.Database.Path)
	if err != nil {
		return err
	}
	store, err := storage.Open(databasePath)
	if err != nil {
		return fmt.Errorf("open plugin state database; stop the server before development sync: %w", err)
	}
	defer store.Close()
	repository, err := plugins.NewSQLiteRepository(store)
	if err != nil {
		return err
	}
	metadata, err := repository.LoadAllPackageMetadata(ctx)
	if err != nil {
		return err
	}
	snapshots = plugins.ApplyPackageMetadata(snapshots, metadata)
	desiredStates, err := repository.LoadDesiredStates(ctx)
	if err != nil {
		return err
	}
	snapshots = plugins.ApplyDesiredStates(snapshots, desiredStates)
	catalog := plugincatalog.New(snapshots)
	registry := tasks.NewRegistry()
	installer, err := pluginservice.NewInstallService(cmd.Logger, registry, catalog, repository, validator, repoRoot, discovery, 15*time.Minute)
	if err != nil {
		return err
	}
	defer installer.Close()
	_, exists := catalog.Get(expectedPluginID)
	request := plugins.InstallRequest{
		SourceType:         "development",
		Source:             sourcePath,
		ResolvedSourceType: "local_directory",
		ResolvedSource:     artifactPath,
		ReplaceExisting:    exists,
	}
	inspection, err := installer.Inspect(ctx, request)
	if err != nil {
		return err
	}
	if inspection.PluginID != expectedPluginID {
		return fmt.Errorf("artifact plugin id %s does not match workspace id %s", inspection.PluginID, expectedPluginID)
	}
	request.InspectionID = inspection.InspectionID
	request.PackageSHA256 = inspection.PackageSHA256
	request.TrustedCodeConfirmed = true
	taskID, err := installer.Accept(ctx, request)
	if err != nil {
		return err
	}
	if err := waitForPluginInstall(ctx, registry, taskID); err != nil {
		return err
	}
	if err := repository.SaveDesiredState(ctx, expectedPluginID, plugins.DesiredStateEnabled, time.Now().UTC()); err != nil {
		return err
	}
	_, _ = catalog.SetDesiredState(expectedPluginID, plugins.DesiredStateEnabled)
	return nil
}

func waitForPluginInstall(ctx context.Context, registry *tasks.Registry, taskID string) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot, ok := registry.Get(taskID)
		if ok {
			switch snapshot.Status {
			case tasks.StatusSucceeded:
				return nil
			case tasks.StatusFailed:
				if snapshot.Error != nil {
					return fmt.Errorf("%s: %s", snapshot.Error.Code, snapshot.Error.Message)
				}
				return fmt.Errorf("plugin installation failed")
			case tasks.StatusCancelled, tasks.StatusInterrupted:
				return fmt.Errorf("plugin installation did not complete: %s", snapshot.Status)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
