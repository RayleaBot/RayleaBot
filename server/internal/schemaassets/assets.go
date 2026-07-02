package schemaassets

import _ "embed"

const (
	ConfigUserSchemaID = "builtin://contracts/config.user.schema.json"
	PluginInfoSchemaID = "builtin://contracts/plugin-info.schema.json"
)

// ConfigUserSchemaJSON mirrors contracts/config.user.schema.json; keep the
// copy in sync via scripts/generate-runtime-schemas.mjs.
//
//go:embed contracts/config.user.schema.json
var ConfigUserSchemaJSON []byte

// PluginInfoSchemaJSON mirrors contracts/plugin-info.schema.json; keep the
// copy in sync via scripts/generate-runtime-schemas.mjs.
//
//go:embed contracts/plugin-info.schema.json
var PluginInfoSchemaJSON []byte

func IsConfigUserSchemaID(name string) bool {
	return name == "" || name == ConfigUserSchemaID
}

func IsPluginInfoSchemaID(name string) bool {
	return name == "" || name == PluginInfoSchemaID
}
