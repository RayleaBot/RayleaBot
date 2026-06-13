package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"gopkg.in/yaml.v3"
)

func ensureDefaultTemplate(configPath string) (map[string]any, error) {
	defaultPath := defaultTemplatePath(configPath)
	rawDefault, exists, err := readYAMLDocument(defaultPath)
	if err != nil {
		return nil, fmt.Errorf("read default config %s: %w", defaultPath, err)
	}

	document := defaultDocument()
	if exists {
		canonicalDefault, err := canonicalizeDocument(rawDefault)
		if err != nil {
			return nil, fmt.Errorf("normalize default config %s: %w", defaultPath, err)
		}
		document = mergeDocuments(document, canonicalDefault)
	}

	if !exists || !reflect.DeepEqual(rawDefault, document) {
		if err := writeCanonicalDocument(defaultPath, document); err != nil {
			return nil, err
		}
	}

	return document, nil
}

func defaultTemplatePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "default.yaml")
}

func readYAMLDocument(path string) (map[string]any, bool, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(bytes, &raw); err != nil {
		return nil, true, fmt.Errorf("parse yaml %s: %w", path, err)
	}
	return raw, true, nil
}

func writeCanonicalDocument(path string, document map[string]any) error {
	yamlBytes, err := yaml.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal config yaml %s: %w", path, err)
	}
	return writeAtomic(path, yamlBytes, 0o644)
}
