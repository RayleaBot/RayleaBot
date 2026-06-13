package config

import (
	"fmt"
	"reflect"
)

const currentSchemaVersion = "2"
const DefaultRenderFooterTemplate = "Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}"
const DefaultRenderOutput = "png"
const DefaultRenderDeviceScalePercent = 100
const DefaultUserCommandRateLimit = "10/60s"
const DefaultGroupCommandRateLimit = "30/60s"
const DefaultCooldownReply = true

func CurrentSchemaVersion() string {
	return currentSchemaVersion
}

func loadCanonicalDocument(configPath, schemaPath string) (map[string]any, Config, error) {
	defaultDoc, err := ensureDefaultTemplate(configPath)
	if err != nil {
		return nil, Config{}, err
	}

	rawUser, userExists, err := readYAMLDocument(configPath)
	if err != nil {
		return nil, Config{}, fmt.Errorf("read config %s: %w", configPath, err)
	}

	userDoc := map[string]any{}
	if userExists {
		userDoc, err = canonicalizeDocument(rawUser)
		if err != nil {
			return nil, Config{}, fmt.Errorf("normalize config document %s: %w", configPath, err)
		}
	}

	document := mergeDocuments(defaultDoc, userDoc)
	if err := validateDocument(schemaPath, document); err != nil {
		return nil, Config{}, fmt.Errorf("config validation failed for %s against %s: %w", configPath, schemaPath, err)
	}

	cfg, err := decodeTypedConfig(document)
	if err != nil {
		return nil, Config{}, fmt.Errorf("decode typed config %s: %w", configPath, err)
	}

	shouldPersist := !userExists || !reflect.DeepEqual(rawUser, document)
	if shouldPersist {
		if err := writeCanonicalDocument(configPath, document); err != nil {
			return nil, Config{}, err
		}
	}

	return document, cfg, nil
}
