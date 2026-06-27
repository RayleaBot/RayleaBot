package cli

import (
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/schemaassets"
)

func validateConfigSchema(schemaPath string) error {
	if schemaassets.IsConfigUserSchemaID(schemaPath) {
		_, err := schema.CompileJSON(schemaassets.ConfigUserSchemaID, schemaassets.ConfigUserSchemaJSON)
		return err
	}
	_, err := schema.Compile(schemaPath)
	return err
}

func displaySchemaPath(repoRoot, schemaPath string) string {
	if schemaPath == "" {
		return schemaassets.ConfigUserSchemaID
	}
	return displayLogPath(repoRoot, schemaPath)
}
