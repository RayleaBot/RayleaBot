package schemaassets

const (
	ConfigUserSchemaID = "builtin://contracts/config.user.schema.json"
	PluginInfoSchemaID = "builtin://contracts/plugin-info.schema.json"
)

func IsConfigUserSchemaID(name string) bool {
	return name == "" || name == ConfigUserSchemaID
}

func IsPluginInfoSchemaID(name string) bool {
	return name == "" || name == PluginInfoSchemaID
}
