package pluginmanifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func manifestDefaultConfig(document map[string]any, packageRoot string) (map[string]any, error) {
	fileConfig, err := manifestDefaultConfigFile(document, packageRoot)
	if err != nil {
		return nil, err
	}

	inlineConfig := manifestObjectField(document, "default_config")
	if len(fileConfig) == 0 {
		return inlineConfig, nil
	}
	if len(inlineConfig) == 0 {
		return fileConfig, nil
	}

	merged := cloneMap(fileConfig)
	for key, value := range inlineConfig {
		merged[key] = cloneValue(value)
	}
	return merged, nil
}

func manifestDefaultConfigFile(document map[string]any, packageRoot string) (map[string]any, error) {
	relativePath := stringField(document, "default_config_file")
	if relativePath == "" {
		return nil, nil
	}
	if filepath.IsAbs(relativePath) {
		return nil, fmt.Errorf("default_config_file must be package-relative")
	}

	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("default_config_file must stay inside the plugin package")
	}
	if filepath.Ext(cleanRelative) != ".json" {
		return nil, fmt.Errorf("default_config_file must point to a .json file")
	}

	packageRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve plugin package root: %w", err)
	}
	configPath := filepath.Join(packageRoot, cleanRelative)
	if !pathWithinRoot(packageRoot, configPath) {
		return nil, fmt.Errorf("default_config_file must stay inside the plugin package")
	}

	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read default_config_file %s: %w", relativePath, err)
	}

	var value any
	if err := json.Unmarshal(bytes, &value); err != nil {
		return nil, fmt.Errorf("parse default_config_file %s: %w", relativePath, err)
	}

	config, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("default_config_file %s must contain a JSON object", relativePath)
	}
	return cloneMap(config), nil
}
