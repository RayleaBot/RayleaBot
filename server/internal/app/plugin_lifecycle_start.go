package app

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (c *pluginLifecycleController) reconcileRuntime(ctx context.Context, botID string) {
	if c == nil || c.plugins == nil {
		return
	}
	botID = strings.TrimSpace(botID)

	for _, snapshot := range c.plugins.List() {
		if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
			continue
		}
		if _, err := c.validateActivation(ctx, snapshot); err != nil {
			c.disablePluginForPermissionLoss(ctx, snapshot.PluginID)
			continue
		}
		if err := c.ensurePluginRunning(ctx, snapshot.PluginID, botID); err != nil {
			c.logLifecycleWarn("plugin runtime reconcile failed", snapshot.PluginID, err)
		}
	}
}

func (c *pluginLifecycleController) ensurePluginRunning(ctx context.Context, pluginID, botID string) error {
	if c == nil || c.runtimes == nil {
		return nil
	}

	manager := c.runtimes.GetOrCreate(pluginID)
	switch manager.Snapshot().State {
	case runtime.StateRunning:
		c.registerRuntimeIfNeeded(pluginID, manager)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
		return nil
	case runtime.StateStarting, runtime.StateStopping, runtime.StateBackoff, runtime.StateCrashed, runtime.StateDeadLetter:
		return nil
	default:
	}

	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStarting))
	return c.startRuntime(ctx, pluginID, botID, manager)
}

func (c *pluginLifecycleController) EnsurePluginRunning(ctx context.Context, pluginID, botID string) error {
	return c.ensurePluginRunning(ctx, pluginID, botID)
}

func (c *pluginLifecycleController) startPluginAsync(pluginID, botID string) {
	if c == nil {
		return
	}
	botID = strings.TrimSpace(botID)

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.state.Config.Runtime))
	defer cancel()

	manager := c.runtimes.GetOrCreate(pluginID)
	if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
		c.logLifecycleWarn("start plugin runtime after enable", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
	}
}

func (c *pluginLifecycleController) reloadPluginAsync(pluginID, botID string, taskID string) {
	if c == nil {
		return
	}
	botID = strings.TrimSpace(botID)
	c.startReloadTask(taskID)

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.state.Config.Runtime))
	defer cancel()

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		c.failReloadTask(taskID, pluginID, "platform.invalid_request", "插件当前不可重载")
		return
	}
	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	current, ok := c.runtimes.Get(pluginID)
	if !ok || current == nil {
		c.updateReloadTask(taskID, 30, "启动插件运行时")
		manager := c.runtimes.GetOrCreate(pluginID)
		if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
			c.logLifecycleWarn("start plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	}

	switch current.Snapshot().State {
	case runtime.StateStopped:
		c.updateReloadTask(taskID, 30, "启动插件运行时")
		if err := c.startRuntime(ctx, pluginID, botID, current); err != nil {
			c.logLifecycleWarn("start stopped plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	case runtime.StateBackoff, runtime.StateCrashed, runtime.StateDeadLetter:
		current.ResetCrashCount()
		current.SetStopped()
		c.updateReloadTask(taskID, 30, "重置插件运行时")
		if err := c.startRuntime(ctx, pluginID, botID, current); err != nil {
			c.logLifecycleWarn("restart plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	case runtime.StateStarting, runtime.StateStopping:
		c.failReloadTask(taskID, pluginID, "platform.invalid_request", "插件运行时正在切换状态")
		return
	}

	c.updateReloadTask(taskID, 30, "构建插件运行时")
	spec, payload, err := c.buildStartInputs(ctx, pluginID, botID)
	if err != nil {
		c.logLifecycleWarn("build runtime spec for plugin reload", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	newManager := c.runtimes.NewDetached()
	c.updateReloadTask(taskID, 60, "重载插件运行时")
	if err := c.dispatcher.ReloadPlugin(ctx, pluginID, current, newManager, spec, payload, dispatchCommands(snapshot.Commands)); err != nil {
		c.logLifecycleWarn("reload plugin runtime", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	c.runtimes.Replace(pluginID, newManager)
	newManager.ResetCrashCount()
	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
	c.clearBotIdentity(pluginID)
	c.afterRuntimeRegistered(ctx, pluginID, botID)
	c.finishReloadTask(taskID, pluginID)
}

func (c *pluginLifecycleController) failReloadTaskForError(taskID string, pluginID string, err error, fallbackMessage string) {
	code := "plugin.internal_error"
	message := fallbackMessage

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		code = "platform.task_timeout"
		message = "插件重载超时"
	}

	var runtimeErr *runtime.Error
	if errors.As(err, &runtimeErr) {
		if strings.TrimSpace(runtimeErr.Code) != "" {
			code = runtimeErr.Code
		}
		if strings.TrimSpace(runtimeErr.Message) != "" {
			message = runtimeErr.Message
		}
	} else {
		var pending *plugins.PermissionPendingError
		if errors.As(err, &pending) {
			code = "plugin.permission_pending"
			message = pending.Error()
		} else if strings.TrimSpace(err.Error()) != "" {
			message = err.Error()
		}
	}

	c.failReloadTask(taskID, pluginID, code, message)
}

func (c *pluginLifecycleController) startRuntime(ctx context.Context, pluginID, botID string, manager *runtime.Manager) error {
	if manager == nil {
		return nil
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	if snapshot.DesiredState != "enabled" {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateStopped))
		return nil
	}

	granted, err := c.validateActivation(ctx, snapshot)
	if err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		return err
	}
	if err := c.seedPluginDefaultConfig(ctx, snapshot); err != nil {
		return err
	}

	spec, payload, err := c.buildStartInputsWithCapabilities(pluginID, botID, granted)
	if err != nil {
		return err
	}

	c.clearBotIdentity(pluginID)
	if err := manager.Start(ctx, spec, payload); err != nil {
		return err
	}

	manager.ResetCrashCount()
	c.registerRuntime(pluginID, snapshot, manager)
	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtime.StateRunning))
	c.afterRuntimeRegistered(ctx, pluginID, botID)
	return nil
}

func (c *pluginLifecycleController) buildStartInputs(ctx context.Context, pluginID, botID string) (runtime.Spec, runtime.InitPayload, error) {
	return c.buildStartInputsWithCapabilities(pluginID, botID, c.grants.grantedCapabilities(ctx, pluginID))
}

func (c *pluginLifecycleController) buildStartInputsWithCapabilities(pluginID, botID string, capabilities []string) (runtime.Spec, runtime.InitPayload, error) {
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return runtime.Spec{}, runtime.InitPayload{}, plugins.ErrPluginNotFound
	}

	spec, err := runtime.BuildSpec(snapshot, c.state.repoRoot, c.state.Config.Runtime)
	if err != nil {
		return runtime.Spec{}, runtime.InitPayload{}, err
	}

	payload := runtime.InitPayload{
		Bot: runtime.BotInfo{
			ID: strings.TrimSpace(botID),
		},
		Capabilities:    append([]string(nil), capabilities...),
		SuperAdmins:     pluginRuntimeSuperAdmins(c.state.Config),
		CommandPrefixes: runtimeCommandPrefixes(c.state.Config),
	}
	return spec, payload, nil
}

func pluginRuntimeSuperAdmins(cfg config.Config) []string {
	source := cfg.Admin.SuperAdmins
	if len(source) == 0 {
		source = cfg.Auth.SuperAdmins
	}
	result := make([]string, 0, len(source))
	seen := make(map[string]struct{}, len(source))
	for _, item := range source {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (c *pluginLifecycleController) afterRuntimeRegistered(ctx context.Context, pluginID string, initBotID string) {
	initBotID = strings.TrimSpace(initBotID)
	currentBotID := c.currentBotID()
	if initBotID != "" {
		c.markBotIdentitySent(pluginID, initBotID)
		if currentBotID != "" && currentBotID != initBotID {
			c.dispatchBotIdentityChangedToPlugin(ctx, pluginID, currentBotID)
		}
		return
	}
	c.dispatchBotIdentityChangedToPlugin(ctx, pluginID, currentBotID)
}

func (c *pluginLifecycleController) registerRuntimeIfNeeded(pluginID string, manager *runtime.Manager) {
	if c == nil || c.dispatcher == nil || manager == nil {
		return
	}
	if c.dispatcher.HasDeliverablePlugin(pluginID) {
		return
	}
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return
	}
	c.registerRuntime(pluginID, snapshot, manager)
}

func (c *pluginLifecycleController) registerRuntime(pluginID string, snapshot plugins.Snapshot, manager *runtime.Manager) {
	if c == nil || c.dispatcher == nil || manager == nil {
		return
	}
	runtimeSnapshot := manager.Snapshot()
	concurrency := snapshot.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if max := c.state.Config.Runtime.MaxConcurrentTasksPerPlugin; max > 0 && concurrency > max {
		concurrency = max
	}
	c.dispatcher.Register(pluginID, manager, runtimeSnapshot.Subscriptions, dispatchCommands(snapshot.Commands), concurrency)
}
