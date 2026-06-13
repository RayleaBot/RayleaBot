package plugins

import (
	"context"
	"time"
)

type DesiredStateRepository interface {
	LoadDesiredStates(context.Context) (map[string]string, error)
	SaveDesiredState(context.Context, string, string, time.Time) error
	DeleteDesiredState(context.Context, string) error
}

type PackageMetadata struct {
	PluginID     string
	SourceType   string
	SourceRef    string
	Version      string
	ManifestHash string
	PackageHash  string
	InstalledAt  time.Time
}

type PackageRepository interface {
	SavePackageMetadata(context.Context, PackageMetadata) error
	DeletePackageMetadata(context.Context, string) error
}

type PackageMetadataLoader interface {
	LoadAllPackageMetadata(context.Context) (map[string]PackageMetadata, error)
}

type PluginGrant struct {
	PluginID   string
	Capability string
	ScopeJSON  string // JSON-encoded scope boundaries (http_hosts, storage_roots, etc.)
	GrantedAt  time.Time
	ExpiresAt  *time.Time
}

type GrantRepository interface {
	LoadGrants(ctx context.Context, pluginID string) ([]PluginGrant, error)
	LoadAllGrants(ctx context.Context) (map[string][]string, error)
	SaveGrant(ctx context.Context, grant PluginGrant) error
	DeleteGrant(ctx context.Context, pluginID, capability string) error
	DeleteAllGrants(ctx context.Context, pluginID string) error
}
