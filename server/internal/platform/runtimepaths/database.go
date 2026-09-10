package runtimepaths

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func DatabaseFromConfig(configPath string) (string, error) {
	payload, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return ResolveDatabasePath(configPath, "data/rayleabot.db")
	}
	if err != nil {
		return "", fmt.Errorf("read database configuration: %w", err)
	}
	var document struct {
		Database struct {
			Path string `yaml:"path"`
		} `yaml:"database"`
	}
	if err := yaml.Unmarshal(payload, &document); err != nil {
		return "", fmt.Errorf("parse database configuration: %w", err)
	}
	databasePath := strings.TrimSpace(document.Database.Path)
	if databasePath == "" {
		databasePath = "data/rayleabot.db"
	}
	return ResolveDatabasePath(configPath, databasePath)
}
