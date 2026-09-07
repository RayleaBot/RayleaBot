package testutil

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// ConfigDocumentOneBot returns the OneBot settings of the adapter a config
// document configures, so a test can edit one transport without restating the
// whole adapters list.
func ConfigDocumentOneBot(t testing.TB, document map[string]any) map[string]any {
	t.Helper()
	return ConfigDocumentAdapter(t, document, config.DefaultOneBot11AdapterID, config.AdapterTypeOneBot11)
}

// ConfigDocumentAdapter returns one adapter instance's settings block.
func ConfigDocumentAdapter(t testing.TB, document map[string]any, id, block string) map[string]any {
	t.Helper()
	adapters, ok := document["adapters"].([]any)
	if !ok {
		t.Fatalf("config document has no adapters list: %#v", document["adapters"])
	}
	for _, entry := range adapters {
		instance, ok := entry.(map[string]any)
		if !ok || instance["id"] != id {
			continue
		}
		settings, ok := instance[block].(map[string]any)
		if !ok {
			t.Fatalf("adapter %q has no %s settings: %#v", id, block, instance)
		}
		return settings
	}
	t.Fatalf("config document has no adapter %q", id)
	return nil
}

// OneBotAdapterDocument builds one adapters entry for a config document.
func OneBotAdapterDocument(id string, enabled bool, settings map[string]any) map[string]any {
	return map[string]any{
		"id":       id,
		"type":     config.AdapterTypeOneBot11,
		"enabled":  enabled,
		"onebot11": settings,
	}
}

// ConfigDocumentAdapterInstance returns one adapter instance itself, for a test
// that needs the instance-level switch rather than its settings.
func ConfigDocumentAdapterInstance(t testing.TB, document map[string]any, id string) map[string]any {
	t.Helper()
	adapters, ok := document["adapters"].([]any)
	if !ok {
		t.Fatalf("config document has no adapters list: %#v", document["adapters"])
	}
	for _, entry := range adapters {
		instance, ok := entry.(map[string]any)
		if ok && instance["id"] == id {
			return instance
		}
	}
	t.Fatalf("config document has no adapter %q", id)
	return nil
}
