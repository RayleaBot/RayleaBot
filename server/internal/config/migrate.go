package config

import (
	"fmt"
	"slices"
	"strings"
)

// documentMigration rewrites a config document from one schema version to the
// next. Migrations run on load, before validation, so a config written by an
// older build keeps working across an upgrade.
type documentMigration struct {
	from string
	to   string
	// apply receives a clone and returns the migrated document. Anything it
	// cannot interpret is left in place rather than dropped, so a partial
	// understanding never silently discards user configuration.
	apply func(map[string]any) (map[string]any, error)
}

var documentMigrations = []documentMigration{
	{from: "3", to: "4", apply: migrateAdaptersToInstances},
}

// LegacyConfigSecretPath returns where a secret lived before the adapters
// migration. Only the two instances that migration creates ever had another
// location; a secret belonging to any other instance has none, so its sealed
// value is only ever stored under its current key.
//
// It is derived by inverting migrateAdaptersToInstances, so the document shape
// and the secret store cannot drift apart, and it lets the store be re-keyed
// from the migrated config alone rather than from the document that produced it.
func LegacyConfigSecretPath(path []string) ([]string, bool) {
	if len(path) < 4 || path[0] != "adapters" {
		return nil, false
	}
	id, block, rest := path[1], path[2], path[3:]
	switch {
	case id == DefaultOneBot11AdapterID && block == AdapterTypeOneBot11 &&
		len(rest) == 2 && slices.Contains(oneBotTransportKeys, rest[0]) && rest[1] == "access_token":
		return []string{"onebot", rest[0], "access_token"}, true
	case id == DefaultQQOfficialAdapterID && block == AdapterTypeQQOfficial &&
		len(rest) == 1 && rest[0] == "app_secret":
		return []string{"qq_official", "app_secret"}, true
	}
	return nil, false
}

var oneBotTransportKeys = []string{"reverse_ws", "forward_ws", "http_api", "webhook"}

// migrateAdaptersToInstances turns the two singleton chat blocks into the
// adapters list. Each becomes one instance keeping its settings, so an upgraded
// install behaves as it did.
func migrateAdaptersToInstances(document map[string]any) (map[string]any, error) {
	migrated := CloneDocument(document)
	instances := make([]any, 0, 2)

	if onebot, ok := migrated["onebot"].(map[string]any); ok {
		settings := CloneDocument(onebot)
		for _, transport := range oneBotTransportKeys {
			rewriteSecretReference(settings, []string{transport, "access_token"},
				[]string{"onebot", transport, "access_token"},
				[]string{"adapters", DefaultOneBot11AdapterID, "onebot11", transport, "access_token"})
		}
		instances = append(instances, map[string]any{
			"id":       DefaultOneBot11AdapterID,
			"type":     AdapterTypeOneBot11,
			"enabled":  anyTransportEnabled(onebot),
			"onebot11": settings,
		})
	}
	if qq, ok := migrated["qq_official"].(map[string]any); ok {
		enabled, _ := qq["enabled"].(bool)
		settings := CloneDocument(qq)
		delete(settings, "enabled")
		rewriteSecretReference(settings, []string{"app_secret"},
			[]string{"qq_official", "app_secret"},
			[]string{"adapters", DefaultQQOfficialAdapterID, "qqofficial", "app_secret"})
		instances = append(instances, map[string]any{
			"id":         DefaultQQOfficialAdapterID,
			"type":       AdapterTypeQQOfficial,
			"enabled":    enabled,
			"qqofficial": settings,
		})
	}

	delete(migrated, "onebot")
	delete(migrated, "qq_official")
	migrated["adapters"] = instances
	return migrated, nil
}

// rewriteSecretReference points a stored secret reference at the field's new
// location. A plaintext value or an empty field is left alone.
func rewriteSecretReference(settings map[string]any, within, fromPath, toPath []string) {
	container := settings
	for _, segment := range within[:len(within)-1] {
		next, ok := container[segment].(map[string]any)
		if !ok {
			return
		}
		container = next
	}
	field := within[len(within)-1]
	value, ok := container[field].(string)
	if !ok {
		return
	}
	if strings.TrimSpace(value) != SecretReferenceFor(fromPath) {
		return
	}
	container[field] = SecretReferenceFor(toPath)
}

// SecretReferenceFor spells the reference a config field uses to point at the
// secret store. It lives here so document migration and the runtime that reads
// these references cannot drift apart.
func SecretReferenceFor(path []string) string {
	return "secret://" + strings.Join(path, "/")
}

// SecretStoreKeyFor spells the secret store key for a config field.
func SecretStoreKeyFor(path []string) string {
	return "config." + strings.Join(path, ".")
}

func anyTransportEnabled(onebot map[string]any) bool {
	for _, key := range oneBotTransportKeys {
		transport, ok := onebot[key].(map[string]any)
		if !ok {
			continue
		}
		if enabled, ok := transport["enabled"].(bool); ok && enabled {
			return true
		}
	}
	return false
}

// MigrateDocument brings a config document up to the current schema version.
// It reports whether anything changed, so callers can persist the result and
// back up what was there before.
func MigrateDocument(document map[string]any) (map[string]any, bool, error) {
	if document == nil {
		return nil, false, nil
	}
	version := strings.TrimSpace(stringValue(document["schema_version"]))
	if version == "" || version == currentSchemaVersion {
		return document, false, nil
	}
	if !CanMigrateFrom(version) {
		return nil, false, fmt.Errorf(
			"config schema_version %q has no migration to supported version %s",
			version, currentSchemaVersion)
	}

	migrated := CloneDocument(document)
	changed := false
	for range documentMigrations {
		current := strings.TrimSpace(stringValue(migrated["schema_version"]))
		if current == currentSchemaVersion {
			break
		}
		step, ok := migrationFrom(current)
		if !ok {
			return nil, false, fmt.Errorf("no config migration from schema_version %q", current)
		}
		next, err := step.apply(migrated)
		if err != nil {
			return nil, false, fmt.Errorf("migrate config from schema_version %s to %s: %w", step.from, step.to, err)
		}
		next["schema_version"] = step.to
		migrated = next
		changed = true
	}

	if final := strings.TrimSpace(stringValue(migrated["schema_version"])); final != currentSchemaVersion {
		return nil, false, fmt.Errorf("config migration stopped at schema_version %q, want %s", final, currentSchemaVersion)
	}
	return migrated, changed, nil
}

func migrationFrom(version string) (documentMigration, bool) {
	for _, migration := range documentMigrations {
		if migration.from == version {
			return migration, true
		}
	}
	return documentMigration{}, false
}

// CanMigrateFrom uses the same implemented path as document import and restore.
func CanMigrateFrom(version string) bool {
	for range len(documentMigrations) + 1 {
		if version == currentSchemaVersion {
			return true
		}
		step, ok := migrationFrom(version)
		if !ok {
			return false
		}
		version = step.to
	}
	return false
}
