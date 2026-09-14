package recovery

import "github.com/RayleaBot/RayleaBot/server/internal/platform/contractversions"

const (
	BackupManifestVersion = contractversions.BackupManifestVersion
	PluginManifestVersion = contractversions.PluginManifestVersion
	PluginProtocolVersion = contractversions.PluginProtocolVersion
	PluginUIBridgeVersion = contractversions.PluginUIBridgeVersion
	PluginArtifactVersion = contractversions.PluginArtifactVersion
)

type BackupManifest struct {
	Version               string                    `json:"version"`
	CreatedAt             string                    `json:"created_at"`
	CoreVersion           string                    `json:"core_version"`
	ConfigSchemaVersion   string                    `json:"config_schema_version"`
	DBSchemaVersion       string                    `json:"db_schema_version"`
	PluginManifestVersion string                    `json:"plugin_manifest_version"`
	PluginProtocolVersion string                    `json:"plugin_protocol_version"`
	PluginArtifactVersion string                    `json:"plugin_artifact_version"`
	PluginUIBridgeVersion string                    `json:"plugin_ui_bridge_version"`
	Consistency           string                    `json:"consistency"`
	Plugins               []BackupManifestPlugin    `json:"plugins,omitempty"`
	Directories           []BackupManifestDirectory `json:"directories,omitempty"`
}

type BackupManifestPlugin struct {
	PluginID        string `json:"plugin_id"`
	ManifestVersion string `json:"manifest_version"`
	ProtocolVersion string `json:"protocol_version"`
	ArtifactVersion string `json:"artifact_version"`
	Version         string `json:"version,omitempty"`
	MinCoreVersion  string `json:"min_core_version,omitempty"`
	SourceRoot      string `json:"source_root,omitempty"`
}

type BackupManifestDirectory struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}
