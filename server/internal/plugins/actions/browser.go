package actions

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/browser"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func browserLaunchRegistrar() registrar {
	return registrar{
		kind: "browser.launch",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeBrowserLaunch(ctx, deps, req)
			}
		},
	}
}

func executeBrowserLaunch(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "browser.launch") {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "browser.launch permission is not declared"}
	}
	if deps.Browser == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "browser session manager is not available"}
	}
	session, err := deps.Browser.Launch(ctx, req.PluginID, browser.LaunchRequest{
		Profile:            req.Action.BrowserProfile,
		Mode:               req.Action.BrowserMode,
		RemoteDebuggingURL: req.Action.BrowserRemoteDebuggingURL,
		LifetimeSeconds:    req.Action.BrowserLifetimeSeconds,
		OwnerDone:          plugins.RuntimeDone(ctx),
	})
	if err != nil {
		return nil, browserActionError(err)
	}
	return map[string]any{
		"session_id":   session.ID,
		"debugger_url": session.DebuggerURL,
		"mode":         session.Mode,
	}, nil
}

func browserCloseRegistrar() registrar {
	return registrar{
		kind: "browser.close",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeBrowserClose(ctx, deps, req)
			}
		},
	}
}

func executeBrowserClose(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "browser.close") {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "browser.close permission is not declared"}
	}
	if deps.Browser == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "browser session manager is not available"}
	}
	closed := deps.Browser.Close(req.PluginID, req.Action.BrowserSessionID)
	return map[string]any{"closed": closed}, nil
}

func browserActionError(err error) error {
	switch {
	case errors.Is(err, browser.ErrBusy):
		return &plugins.Error{Code: errorcodes.PlatformResourceBusy, Message: "browser profile is busy", Err: err}
	case errors.Is(err, browser.ErrInvalidRequest):
		return &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "browser launch request is invalid", Err: err}
	case errors.Is(err, browser.ErrUnavailable):
		return &plugins.Error{Code: errorcodes.PlatformResourceMissing, Message: "browser is unavailable", Err: err}
	default:
		return &plugins.Error{Code: errorcodes.PluginInternalError, Message: "browser launch failed", Err: err}
	}
}
