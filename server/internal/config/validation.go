package config

import (
	"encoding/json"
)

func normalizeDocument(raw map[string]any) (any, error) {
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	var document any
	if err := json.Unmarshal(jsonBytes, &document); err != nil {
		return nil, err
	}

	return document, nil
}

func validateDocument(schemaPath string, document any) error {
	if IsConfigUserSchemaID(schemaPath) {
		validator, err := CompileJSON(ConfigUserSchemaID, ConfigUserSchemaJSON)
		if err != nil {
			return err
		}
		return validator.Validate(document)
	}

	validator, err := Compile(schemaPath)
	if err != nil {
		return err
	}

	if err := validator.Validate(document); err != nil {
		return err
	}

	return nil
}
