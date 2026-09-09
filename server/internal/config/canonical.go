package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

const currentSchemaVersion = "4"
const DefaultRenderFooterTemplate = "Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}"
const DefaultRenderOutput = "png"
const DefaultRenderDeviceScalePercent = 100
const DefaultUserCommandRateLimit = "10/60s"
const DefaultGroupCommandRateLimit = "30/60s"
const DefaultCooldownReply = true

func CurrentSchemaVersion() string {
	return currentSchemaVersion
}

// loadCanonicalDocument reads and validates the config. persistMigration says
// whether a document the migration changed is written back; callers that only
// inspect the config leave the file untouched.
func loadCanonicalDocument(configPath, schemaPath string, persistMigration bool) (map[string]any, Config, error) {
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
		// Migration runs before canonicalisation and validation: a config
		// written by an older build must reach the current shape before
		// anything judges it against the current schema.
		migrated, changed, err := MigrateDocument(rawUser)
		if err != nil {
			return nil, Config{}, fmt.Errorf("migrate config %s: %w", configPath, err)
		}
		if changed && persistMigration {
			if err := backupConfigBeforeMigration(configPath); err != nil {
				return nil, Config{}, fmt.Errorf("back up config before migration %s: %w", configPath, err)
			}
		}
		userDoc, err = canonicalizeDocument(migrated)
		if err != nil {
			return nil, Config{}, fmt.Errorf("normalize config document %s: %w", configPath, err)
		}
		if changed && persistMigration {
			if err := writeCanonicalDocument(configPath, userDoc); err != nil {
				return nil, Config{}, fmt.Errorf("persist migrated config %s: %w", configPath, err)
			}
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
		// Normalising rewrites the file anyway, so a config from an older build
		// is migrated first rather than failing the current schema.
		migrated, changed, err := MigrateDocument(rawUser)
		if err != nil {
			return Config{}, Summary{}, fmt.Errorf("migrate config %s: %w", configPath, err)
		}
		if changed {
			if err := backupConfigBeforeMigration(configPath); err != nil {
				return Config{}, Summary{}, fmt.Errorf("back up config before migration %s: %w", configPath, err)
			}
		}
		userDoc, err = canonicalizeDocument(migrated)
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
		canonicalDefault, err := migrateAndCanonicalize(defaultPath, rawDefault)
		if err != nil {
			return nil, err
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
		canonicalDefault, err := migrateAndCanonicalize(defaultPath, rawDefault)
		if err != nil {
			return nil, err
		}
		document = mergeDocuments(document, canonicalDefault)
	}
	return document, nil
}

// migrateAndCanonicalize brings the shipped default template up to the current
// shape before it is merged. The template is a config document like any other:
// an upgrade that leaves it behind would reintroduce the old sections into
// every merge, and the merged document would then fail the current schema.
func migrateAndCanonicalize(path string, raw map[string]any) (map[string]any, error) {
	migrated, _, err := MigrateDocument(raw)
	if err != nil {
		return nil, fmt.Errorf("migrate default config %s: %w", path, err)
	}
	canonical, err := canonicalizeDocument(migrated)
	if err != nil {
		return nil, fmt.Errorf("normalize default config %s: %w", path, err)
	}
	return canonical, nil
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

// backupConfigBeforeMigration keeps the pre-migration file beside the config so
// a migration that turns out wrong is recoverable by hand. An existing backup is
// never overwritten: the first one is the original.
func backupConfigBeforeMigration(configPath string) error {
	backupPath := configPath + ".pre-migration.bak"
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	return os.WriteFile(backupPath, contents, 0o600)
}
