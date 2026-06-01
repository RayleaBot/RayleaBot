package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

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
