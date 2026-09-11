package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/contractversions"
	"gopkg.in/yaml.v3"
)

const currentSchemaVersion = contractversions.ConfigSchemaVersion
const DefaultRenderFooterTemplate = "Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}"
const DefaultRenderOutput = "png"
const DefaultRenderDeviceScalePercent = 100
const DefaultUserCommandRateLimit = "10/60s"
const DefaultGroupCommandRateLimit = "30/60s"

func CurrentSchemaVersion() string {
	return currentSchemaVersion
}

// loadCanonicalDocument combines the embedded defaults with the user's document.
// Loading and validation never change files; Init and Normalize persist the result.
func loadCanonicalDocument(configPath, schemaPath string) (map[string]any, Config, error) {
	rawUser, _, err := readYAMLDocument(configPath)
	if err != nil {
		return nil, Config{}, fmt.Errorf("read config %s: %w", configPath, err)
	}
	if rawUser == nil {
		rawUser = map[string]any{}
	}
	userDoc, err := canonicalizeDocument(rawUser)
	if err != nil {
		return nil, Config{}, fmt.Errorf("normalize config document %s: %w", configPath, err)
	}
	document := mergeDocuments(defaultDocument(), userDoc)
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
	document, cfg, err := loadCanonicalDocument(configPath, schemaPath)
	if err != nil {
		return Config{}, Summary{}, err
	}
	if err := writeCanonicalDocument(configPath, document); err != nil {
		return Config{}, Summary{}, err
	}
	return cfg, buildSummary(configPath, schemaPath, cfg, document), nil
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
	yamlBytes, err := MarshalDocument(document)
	if err != nil {
		return fmt.Errorf("marshal config yaml %s: %w", path, err)
	}
	return writeAtomic(path, yamlBytes, 0o644)
}

// MarshalDocument encodes a configuration document with stable YAML integers.
func MarshalDocument(document map[string]any) ([]byte, error) {
	return yaml.Marshal(yamlDocumentValue(document))
}

// JSON document numbers are float64. Render integral values as YAML integers so
// repeated init, edit and normalize operations keep a stable numeric form.
func yamlDocumentValue(value any) any {
	switch typed := value.(type) {
	case float64:
		if typed == math.Trunc(typed) && typed >= math.MinInt64 && typed < math.MaxInt64 {
			return int64(typed)
		}
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = yamlDocumentValue(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = yamlDocumentValue(child)
		}
		return result
	}
	return value
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
	cfg.Scheduler.Timezone = NormalizeTimezone(cfg.Scheduler.Timezone)
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

// normalizeOneBotSection fills each OneBot adapter's transports. The settings
// live inside the adapters list, so every instance is normalized, not just one.
func normalizeOneBotSection(document map[string]any) {
	adapters, ok := document["adapters"].([]any)
	if !ok {
		return
	}
	for _, entry := range adapters {
		instance, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		onebot, ok := instance["onebot11"].(map[string]any)
		if !ok {
			continue
		}
		normalizeOneBotTransport(onebot, "reverse_ws", true)
		normalizeOneBotTransport(onebot, "forward_ws", true)
		normalizeOneBotTransport(onebot, "http_api", false)
		normalizeOneBotTransport(onebot, "webhook", true)
	}
}

func normalizeOneBotTransport(onebot map[string]any, key string, allowQueryCompat bool) {
	transport := transportSection(onebot, key)
	if transport == nil {
		if _, present := onebot[key]; present {
			return
		}
		transport = map[string]any{
			"enabled": false,
			"url":     "",
		}
		onebot[key] = transport
	}

	if value, ok := transport["url"].(string); ok {
		transport["url"] = strings.TrimSpace(value)
	} else if _, present := transport["url"]; !present {
		transport["url"] = ""
	}
	if _, present := transport["enabled"]; !present {
		transport["enabled"] = false
	}
	if value, ok := transport["access_token"].(string); ok {
		transport["access_token"] = strings.TrimSpace(value)
	} else if _, present := transport["access_token"]; !present {
		transport["access_token"] = ""
	}
	if allowQueryCompat {
		if _, present := transport["access_token_query_compat"]; !present {
			transport["access_token_query_compat"] = false
		}
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
