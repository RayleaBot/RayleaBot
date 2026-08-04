package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "embed"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	ConfigUserSchemaID           = "builtin://contracts/config.user.schema.json"
	PluginInfoSchemaID           = "builtin://contracts/plugin-info.schema.json"
	PluginArtifactSchemaID       = "builtin://contracts/plugin-artifact.schema.json"
	PluginStoreCatalogSchemaID   = "builtin://contracts/plugin-store-catalog.schema.json"
	PluginStoreSignatureSchemaID = "builtin://contracts/plugin-store-signature.schema.json"
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

// PluginArtifactSchemaJSON mirrors contracts/plugin-artifact.schema.json; keep
// the copy in sync via scripts/generate-runtime-schemas.mjs.
//
//go:embed contracts/plugin-artifact.schema.json
var PluginArtifactSchemaJSON []byte

// PluginStoreCatalogSchemaJSON mirrors contracts/plugin-store-catalog.schema.json.
//
//go:embed contracts/plugin-store-catalog.schema.json
var PluginStoreCatalogSchemaJSON []byte

// PluginStoreSignatureSchemaJSON mirrors contracts/plugin-store-signature.schema.json.
//
//go:embed contracts/plugin-store-signature.schema.json
var PluginStoreSignatureSchemaJSON []byte

func IsConfigUserSchemaID(name string) bool {
	return name == "" || name == ConfigUserSchemaID
}

func IsPluginInfoSchemaID(name string) bool {
	return name == "" || name == PluginInfoSchemaID
}

type Validator struct {
	path   string
	schema *jsonschema.Schema
}

func Compile(path string) (*Validator, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve schema path %s: %w", path, err)
	}

	document, err := LoadJSONFile(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("load schema %s: %w", absolutePath, err)
	}

	return compileDocument(absolutePath, document)
}

func CompileDocument(name string, document any) (*Validator, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("schema name is required")
	}

	return compileDocument(name, document)
}

func CompileJSON(name string, content []byte) (*Validator, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("schema name is required")
	}

	var document any
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("unmarshal schema %s: %w", name, err)
	}

	return compileDocument(name, document)
}

func (v *Validator) Path() string {
	return v.path
}

func (v *Validator) Validate(document any) error {
	if err := v.schema.Validate(document); err != nil {
		return err
	}

	return nil
}

func LoadJSONFile(path string) (any, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read json %s: %w", path, err)
	}

	var document any
	if err := json.Unmarshal(bytes, &document); err != nil {
		return nil, fmt.Errorf("unmarshal json %s: %w", path, err)
	}

	return document, nil
}

func compileDocument(name string, document any) (*Validator, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(name, document); err != nil {
		return nil, fmt.Errorf("add schema resource %s: %w", name, err)
	}

	compiledSchema, err := compiler.Compile(name)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", name, err)
	}

	return &Validator{
		path:   name,
		schema: compiledSchema,
	}, nil
}

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
