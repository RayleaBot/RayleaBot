package cli

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func validateConfigSchema(schemaPath string) error {
	if config.IsConfigUserSchemaID(schemaPath) {
		_, err := config.CompileJSON(config.ConfigUserSchemaID, config.ConfigUserSchemaJSON)
		return err
	}
	_, err := config.Compile(schemaPath)
	return err
}

func displaySchemaPath(repoRoot, schemaPath string) string {
	if schemaPath == "" {
		return config.ConfigUserSchemaID
	}
	return displayLogPath(repoRoot, schemaPath)
}
