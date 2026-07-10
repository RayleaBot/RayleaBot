package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
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
	defaultDoc, err := readDefaultTemplate(configPath)
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
	if err := validateRuntimeConstraints(cfg); err != nil {
		return nil, Config{}, fmt.Errorf("config runtime constraints failed for %s: %w", configPath, err)
	}

	return document, cfg, nil
}

func normalizeCanonicalDocument(configPath, schemaPath string) (Config, Summary, error) {
	defaultDoc, err := ensureDefaultTemplate(configPath)
	if err != nil {
		return Config{}, Summary{}, err
	}

	rawUser, userExists, err := readYAMLDocument(configPath)
	if err != nil {
		return Config{}, Summary{}, fmt.Errorf("read config %s: %w", configPath, err)
	}

	userDoc := map[string]any{}
	if userExists {
		userDoc, err = canonicalizeDocument(rawUser)
		if err != nil {
			return Config{}, Summary{}, fmt.Errorf("normalize config document %s: %w", configPath, err)
		}
	}

	document := mergeDocuments(defaultDoc, userDoc)
	if err := validateDocument(schemaPath, document); err != nil {
		return Config{}, Summary{}, fmt.Errorf("config validation failed for %s against %s: %w", configPath, schemaPath, err)
	}

	cfg, err := decodeTypedConfig(document)
	if err != nil {
		return Config{}, Summary{}, fmt.Errorf("decode typed config %s: %w", configPath, err)
	}
	if err := validateRuntimeConstraints(cfg); err != nil {
		return Config{}, Summary{}, fmt.Errorf("config runtime constraints failed for %s: %w", configPath, err)
	}
	if err := writeCanonicalDocument(configPath, document); err != nil {
		return Config{}, Summary{}, err
	}

	return cfg, buildSummary(configPath, schemaPath, cfg, document), nil
}

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

func readDefaultTemplate(configPath string) (map[string]any, error) {
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

func canonicalizeDocument(raw map[string]any) (map[string]any, error) {
	normalized, err := normalizeDocument(raw)
	if err != nil {
		return nil, err
	}

	document, ok := normalized.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("normalized document is not an object")
	}
	document = stripNullValues(document)

	cloned := CloneDocument(document)
	if cloned == nil {
		cloned = map[string]any{}
	}
	if version := strings.TrimSpace(stringValue(cloned["schema_version"])); version == "" {
		cloned["schema_version"] = currentSchemaVersion
	}
	normalizeOneBotSection(cloned)
	return cloned, nil
}

func stripNullValues(document map[string]any) map[string]any {
	if document == nil {
		return nil
	}

	cleaned := make(map[string]any, len(document))
	for key, value := range document {
		cleanedValue, keep := stripNullValue(value)
		if !keep {
			continue
		}
		cleaned[key] = cleanedValue
	}
	return cleaned
}

func stripNullValue(value any) (any, bool) {
	if value == nil {
		return nil, false
	}

	switch typed := value.(type) {
	case map[string]any:
		return stripNullValues(typed), true
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			cleanedItem, keep := stripNullValue(item)
			if !keep {
				continue
			}
			items = append(items, cleanedItem)
		}
		return items, true
	default:
		return value, true
	}
}

func section(document map[string]any, key string) map[string]any {
	value, ok := document[key]
	if !ok {
		return nil
	}
	typed, _ := value.(map[string]any)
	return typed
}

func transportSection(document map[string]any, key string) map[string]any {
	value, ok := document[key]
	if !ok {
		return nil
	}
	typed, _ := value.(map[string]any)
	return typed
}

func mergeDocuments(base, overlay map[string]any) map[string]any {
	result := CloneDocument(base)
	if result == nil {
		result = map[string]any{}
	}
	for key, value := range overlay {
		targetSection, targetIsMap := result[key].(map[string]any)
		sourceSection, sourceIsMap := value.(map[string]any)
		if targetIsMap && sourceIsMap {
			result[key] = mergeDocuments(targetSection, sourceSection)
			continue
		}
		result[key] = cloneValue(value)
	}
	return result
}

func cloneValue(value any) any {
	bytes, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned any
	if err := json.Unmarshal(bytes, &cloned); err != nil {
		return value
	}
	return cloned
}

func decodeTypedConfig(document map[string]any) (Config, error) {
	var cfg Config
	jsonBytes, err := json.Marshal(document)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(jsonBytes, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func normalizeOneBotSection(document map[string]any) {
	onebot := section(document, "onebot")
	if onebot == nil {
		return
	}

	normalizeOneBotTransport(onebot, "reverse_ws", true)
	normalizeOneBotTransport(onebot, "forward_ws", true)
	normalizeOneBotTransport(onebot, "http_api", false)
	normalizeOneBotTransport(onebot, "webhook", true)
}

func normalizeOneBotTransport(onebot map[string]any, key string, allowQueryCompat bool) {
	transport := transportSection(onebot, key)
	if transport == nil {
		transport = map[string]any{
			"enabled": false,
			"url":     "",
		}
		onebot[key] = transport
	}

	urlValue := strings.TrimSpace(stringValue(transport["url"]))
	transport["url"] = urlValue
	if _, ok := transport["enabled"].(bool); !ok {
		transport["enabled"] = false
	}
	transport["access_token"] = strings.TrimSpace(stringValue(transport["access_token"]))
	if allowQueryCompat {
		if _, ok := transport["access_token_query_compat"].(bool); !ok {
			transport["access_token_query_compat"] = false
		}
	} else {
		delete(transport, "access_token_query_compat")
	}
}

func oneBotTransportDocument(enabled bool, urlValue string, accessToken string) map[string]any {
	return map[string]any{
		"enabled":      enabled,
		"url":          urlValue,
		"access_token": accessToken,
	}
}

func oneBotTransportConfigDocument(transport OneBotTransportConfig) map[string]any {
	return oneBotTransportDocument(transport.Enabled, transport.URL, transport.AccessToken)
}

func oneBotTransportCompatDocument(transport OneBotTransportConfig) map[string]any {
	document := oneBotTransportDocument(transport.Enabled, transport.URL, transport.AccessToken)
	document["access_token_query_compat"] = transport.AccessTokenQueryCompat
	return document
}
