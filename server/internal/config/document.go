package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func LoadDocument(configPath, schemaPath string) (map[string]any, error) {
	document, _, err := loadCanonicalDocument(configPath, schemaPath)
	if err != nil {
		return nil, err
	}
	return CloneDocument(document), nil
}

func SaveDocument(configPath, schemaPath string, document map[string]any) (Config, Summary, error) {
	cfg, summary, canonical, err := NormalizeDocument(configPath, schemaPath, document)
	if err != nil {
		return Config{}, Summary{}, err
	}
	if err := writeCanonicalDocument(configPath, canonical); err != nil {
		return Config{}, Summary{}, err
	}
	return cfg, summary, nil
}

func NormalizeDocument(configPath, schemaPath string, document map[string]any) (Config, Summary, map[string]any, error) {
	canonical, err := canonicalizeDocument(document)
	if err != nil {
		return Config{}, Summary{}, nil, fmt.Errorf("normalize config document %s: %w", configPath, err)
	}
	if err := validateDocument(schemaPath, canonical); err != nil {
		return Config{}, Summary{}, nil, fmt.Errorf("config validation failed for %s against %s: %w", configPath, schemaPath, err)
	}
	cfg, err := decodeTypedConfig(canonical)
	if err != nil {
		return Config{}, Summary{}, nil, fmt.Errorf("decode typed config %s: %w", configPath, err)
	}
	return cfg, buildSummary(configPath, schemaPath, cfg, canonical), canonical, nil
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
