package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadDocument(configPath, schemaPath string) (map[string]any, error) {
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", configPath, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(configBytes, &raw); err != nil {
		return nil, fmt.Errorf("parse yaml %s: %w", configPath, err)
	}

	document, err := normalizeDocument(raw)
	if err != nil {
		return nil, fmt.Errorf("normalize config document %s: %w", configPath, err)
	}
	if err := validateDocument(schemaPath, document); err != nil {
		return nil, fmt.Errorf("config validation failed for %s against %s: %w", configPath, schemaPath, err)
	}

	typed, ok := document.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("normalized config %s is not an object", configPath)
	}

	return CloneDocument(typed), nil
}

func SaveDocument(configPath, schemaPath string, document map[string]any) (Config, Summary, error) {
	var cfg Config

	normalized, err := normalizeDocument(document)
	if err != nil {
		return cfg, Summary{}, fmt.Errorf("normalize config document %s: %w", configPath, err)
	}
	if err := validateDocument(schemaPath, normalized); err != nil {
		return cfg, Summary{}, fmt.Errorf("config validation failed for %s against %s: %w", configPath, schemaPath, err)
	}

	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		return cfg, Summary{}, fmt.Errorf("marshal normalized config %s: %w", configPath, err)
	}
	if err := json.Unmarshal(jsonBytes, &cfg); err != nil {
		return cfg, Summary{}, fmt.Errorf("decode typed config %s: %w", configPath, err)
	}

	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return cfg, Summary{}, fmt.Errorf("marshal config yaml %s: %w", configPath, err)
	}

	if err := writeAtomic(configPath, yamlBytes, 0o644); err != nil {
		return cfg, Summary{}, err
	}

	return cfg, buildSummary(configPath, schemaPath, cfg), nil
}

func writeAtomic(path string, contents []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %w", filepath.Dir(path), err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary config file for %s: %w", path, err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(contents); err != nil {
		tempFile.Close()
		return fmt.Errorf("write temporary config file for %s: %w", path, err)
	}
	if err := tempFile.Chmod(mode); err != nil {
		tempFile.Close()
		return fmt.Errorf("set mode on temporary config file for %s: %w", path, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary config file for %s: %w", path, err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config file %s: %w", path, err)
	}

	return nil
}

func CloneDocument(document map[string]any) map[string]any {
	if document == nil {
		return nil
	}

	jsonBytes, err := json.Marshal(document)
	if err != nil {
		return nil
	}

	var cloned map[string]any
	if err := json.Unmarshal(jsonBytes, &cloned); err != nil {
		return nil
	}
	return cloned
}
