package plugins

import "context"

type InstallRequest struct {
	SourceType          string
	Source              string
	AllowInstallScripts bool
}

type InstallCoordinator interface {
	Accept(context.Context, InstallRequest) (string, error)
	Cancel(string) bool
	Close() error
}

type StopPluginFunc func(context.Context, string)

type UninstallCoordinator interface {
	Accept(ctx context.Context, pluginID string) (string, error)
}
