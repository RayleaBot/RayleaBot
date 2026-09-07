package configruntime

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

func TestSanitizeConfigDocumentRedactsEveryConfigSecret(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		"adapters": []any{
			map[string]any{"id": "onebot11", "type": "onebot11", "onebot11": map[string]any{
				"forward_ws": map[string]any{"access_token": "forward-secret"},
				"http_api":   map[string]any{"access_token": "http-secret"},
				"reverse_ws": map[string]any{"access_token": "reverse-secret"},
				"webhook":    map[string]any{"access_token": "webhook-secret"},
			}},
			map[string]any{"id": "qq-official", "type": "qqofficial", "qqofficial": map[string]any{
				"app_secret": "qq-app-secret",
			}},
		},
	}

	redacted, fields := sanitizeConfigDocument(document)
	wantFields := []string{
		"adapters.onebot11.onebot11.forward_ws.access_token",
		"adapters.onebot11.onebot11.http_api.access_token",
		"adapters.onebot11.onebot11.reverse_ws.access_token",
		"adapters.onebot11.onebot11.webhook.access_token",
		"adapters.qq-official.qqofficial.app_secret",
	}
	if !reflect.DeepEqual(fields, wantFields) {
		t.Fatalf("redacted fields = %#v, want %#v", fields, wantFields)
	}
	for _, path := range configSecretPathsIn(redacted) {
		if got := stringAtPath(t, redacted, path); got != redactedConfigValue {
			t.Fatalf("%v = %q, want redacted marker", path, got)
		}
	}
	if got := stringAtPath(t, document, onebotSecretPath("onebot11", "forward_ws")); got != "forward-secret" {
		t.Fatalf("original document was mutated: %q", got)
	}
}

// Two instances of the same adapter type hold separate credentials, so each
// entry has to redact into its own path rather than collapsing onto the type.
func TestSanitizeConfigDocumentSeparatesInstancesOfOneType(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		"adapters": []any{
			adapterDocument("primary", "onebot11", map[string]any{
				"reverse_ws": map[string]any{"access_token": "primary-secret"},
			}),
			adapterDocument("secondary", "onebot11", map[string]any{
				"reverse_ws": map[string]any{"access_token": "secondary-secret"},
			}),
		},
	}

	_, fields := sanitizeConfigDocument(document)
	want := []string{
		"adapters.primary.onebot11.reverse_ws.access_token",
		"adapters.secondary.onebot11.reverse_ws.access_token",
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("redacted fields = %#v, want %#v", fields, want)
	}
}

func TestRestoreRedactedConfigSecretsRetainsReplacesAndClears(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"adapters": []any{adapterDocument("onebot11", "onebot11", map[string]any{
			"forward_ws": map[string]any{"access_token": "old-forward"},
			"http_api":   map[string]any{"access_token": "old-http"},
			"reverse_ws": map[string]any{"access_token": "old-reverse"},
			"webhook":    map[string]any{"access_token": "old-webhook"},
		})},
	}
	request := map[string]any{
		"adapters": []any{adapterDocument("onebot11", "onebot11", map[string]any{
			"forward_ws": map[string]any{"access_token": "new-forward"},
			"http_api":   map[string]any{"access_token": ""},
			"reverse_ws": map[string]any{"access_token": redactedConfigValue},
		})},
	}

	restored := restoreRedactedConfigSecrets(request, current)
	assertStringAtPath(t, restored, onebotSecretPath("onebot11", "forward_ws"), "new-forward")
	assertStringAtPath(t, restored, onebotSecretPath("onebot11", "http_api"), "")
	assertStringAtPath(t, restored, onebotSecretPath("onebot11", "reverse_ws"), "old-reverse")
	assertStringAtPath(t, restored, onebotSecretPath("onebot11", "webhook"), "old-webhook")
}

// Secrets belong to the instance that holds them: a redacted marker under one
// id must never be filled from another instance of the same adapter type.
func TestRestoreRedactedConfigSecretsKeepsInstancesApart(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"adapters": []any{
			adapterDocument("primary", "onebot11", map[string]any{
				"reverse_ws": map[string]any{"access_token": "primary-secret"},
			}),
			adapterDocument("secondary", "onebot11", map[string]any{
				"reverse_ws": map[string]any{"access_token": "secondary-secret"},
			}),
		},
	}
	// The request reorders the instances, which must not move their secrets.
	request := map[string]any{
		"adapters": []any{
			adapterDocument("secondary", "onebot11", map[string]any{
				"reverse_ws": map[string]any{"access_token": redactedConfigValue},
			}),
			adapterDocument("primary", "onebot11", map[string]any{
				"reverse_ws": map[string]any{"access_token": redactedConfigValue},
			}),
		},
	}

	restored := restoreRedactedConfigSecrets(request, current)
	assertStringAtPath(t, restored, onebotSecretPath("primary", "reverse_ws"), "primary-secret")
	assertStringAtPath(t, restored, onebotSecretPath("secondary", "reverse_ws"), "secondary-secret")
}

func TestApplyHotReloadableFieldsAddsConfigSecretsToRedactor(t *testing.T) {
	t.Parallel()

	var added []string
	service := &Service{
		currentConfig: func() internalconfig.Config {
			return internalconfig.Config{}
		},
		addRedactionValues: func(values ...string) {
			added = append(added, values...)
		},
	}
	next := internalconfig.Config{Adapters: []internalconfig.AdapterInstance{{
		ID: "onebot11", Type: internalconfig.AdapterTypeOneBot11, Enabled: true,
		OneBot11: &internalconfig.OneBotConfig{
			ForwardWS: internalconfig.OneBotTransportConfig{AccessToken: "forward-secret"},
			HTTPAPI:   internalconfig.OneBotTransportConfig{AccessToken: "http-secret"},
			ReverseWS: internalconfig.OneBotTransportConfig{AccessToken: "reverse-secret"},
			Webhook:   internalconfig.OneBotTransportConfig{AccessToken: "webhook-secret"},
		},
	}}}

	service.ApplyHotReloadableFields(next)
	want := []string{"forward-secret", "http-secret", "reverse-secret", "webhook-secret"}
	if !reflect.DeepEqual(added, want) {
		t.Fatalf("added redaction values = %#v, want %#v", added, want)
	}
}

func TestStoreConfigSecretsSealsEveryConfigSecret(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemorySecretStore()
	document := map[string]any{
		"adapters": []any{
			map[string]any{"id": "onebot11", "type": "onebot11", "onebot11": map[string]any{
				"forward_ws": map[string]any{"access_token": "forward-secret"},
				"http_api":   map[string]any{"access_token": "http-secret"},
				"reverse_ws": map[string]any{"access_token": "reverse-secret"},
				"webhook":    map[string]any{"access_token": "webhook-secret"},
			}},
			map[string]any{"id": "qq-official", "type": "qqofficial", "qqofficial": map[string]any{
				"app_secret": "qq-app-secret",
			}},
		},
	}

	stored, err := StoreConfigSecrets(ctx, store, document)
	if err != nil {
		t.Fatalf("store config secrets: %v", err)
	}
	for _, path := range configSecretPathsIn(document) {
		if got := stringAtPath(t, stored, path); got != configSecretReference(path) {
			t.Fatalf("%v = %q, want %q", path, got, configSecretReference(path))
		}
		if got := stringAtPath(t, document, path); got == configSecretReference(path) {
			t.Fatalf("original document was mutated at %v", path)
		}
	}

	storedForward, err := store.Get(ctx, configSecretKey(onebotSecretPath("onebot11", "forward_ws")))
	if err != nil {
		t.Fatalf("read stored forward secret: %v", err)
	}
	if string(storedForward) == "forward-secret" {
		t.Fatal("forward secret was stored as plaintext")
	}
	openedForward, err := secrets.OpenString(ctx, store, storedForward)
	if err != nil {
		t.Fatalf("open stored forward secret: %v", err)
	}
	if openedForward != "forward-secret" {
		t.Fatalf("opened forward secret = %q, want forward-secret", openedForward)
	}

	adapterPath := func(transport string) []string {
		return onebotSecretPath("onebot11", transport)
	}
	resolved, err := ResolveConfigSecretRefs(ctx, store, internalconfig.Config{Adapters: []internalconfig.AdapterInstance{{
		ID: "onebot11", Type: internalconfig.AdapterTypeOneBot11, Enabled: true,
		OneBot11: &internalconfig.OneBotConfig{
			ForwardWS: internalconfig.OneBotTransportConfig{AccessToken: configSecretReference(adapterPath("forward_ws"))},
			HTTPAPI:   internalconfig.OneBotTransportConfig{AccessToken: configSecretReference(adapterPath("http_api"))},
			ReverseWS: internalconfig.OneBotTransportConfig{AccessToken: configSecretReference(adapterPath("reverse_ws"))},
			Webhook:   internalconfig.OneBotTransportConfig{AccessToken: configSecretReference(adapterPath("webhook"))},
		},
	}}})
	if err != nil {
		t.Fatalf("resolve config secret refs: %v", err)
	}
	want := internalconfig.OneBotConfig{
		ForwardWS: internalconfig.OneBotTransportConfig{AccessToken: "forward-secret"},
		HTTPAPI:   internalconfig.OneBotTransportConfig{AccessToken: "http-secret"},
		ReverseWS: internalconfig.OneBotTransportConfig{AccessToken: "reverse-secret"},
		Webhook:   internalconfig.OneBotTransportConfig{AccessToken: "webhook-secret"},
	}
	if !reflect.DeepEqual(mustOneBot(t, resolved), want) {
		t.Fatalf("resolved onebot = %#v, want %#v", mustOneBot(t, resolved), want)
	}
}

func TestStoreConfigSecretsDeletesClearedToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemorySecretStore()
	path := onebotSecretPath("onebot11", "forward_ws")
	sealed, err := secrets.SealString(ctx, store, "forward-secret")
	if err != nil {
		t.Fatalf("seal fixture secret: %v", err)
	}
	if err := store.Set(ctx, configSecretKey(path), sealed); err != nil {
		t.Fatalf("store fixture secret: %v", err)
	}

	_, err = StoreConfigSecrets(ctx, store, map[string]any{
		"adapters": []any{adapterDocument("onebot11", "onebot11", map[string]any{
			"forward_ws": map[string]any{"access_token": ""},
		})},
	})
	if err != nil {
		t.Fatalf("store config secrets: %v", err)
	}
	if _, err := store.Get(ctx, configSecretKey(path)); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("cleared secret lookup error = %v, want ErrNotFound", err)
	}
}

func TestResolveConfigSecretRefsRejectsWrongReference(t *testing.T) {
	t.Parallel()

	_, err := ResolveConfigSecretRefs(context.Background(), newMemorySecretStore(), internalconfig.Config{
		Adapters: []internalconfig.AdapterInstance{{
			ID: "onebot11", Type: internalconfig.AdapterTypeOneBot11, Enabled: true,
			OneBot11: &internalconfig.OneBotConfig{
				ForwardWS: internalconfig.OneBotTransportConfig{
					AccessToken: configSecretReference(onebotSecretPath("onebot11", "reverse_ws")),
				},
			},
		}},
	})
	if err == nil {
		t.Fatal("expected wrong secret reference to fail")
	}
}

// adapterDocument builds one entry of the adapters collection.
func adapterDocument(id, adapterType string, settings map[string]any) map[string]any {
	return map[string]any{"id": id, "type": adapterType, adapterType: settings}
}

func onebotSecretPath(id, transport string) []string {
	return []string{"adapters", id, "onebot11", transport, "access_token"}
}

func assertStringAtPath(t *testing.T, document map[string]any, path []string, want string) {
	t.Helper()
	if got := stringAtPath(t, document, path); got != want {
		t.Fatalf("%v = %q, want %q", path, got, want)
	}
}

func stringAtPath(t *testing.T, document map[string]any, path []string) string {
	t.Helper()
	value, ok := lookupConfigPath(document, path)
	if !ok {
		t.Fatalf("missing path %v", path)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("path %v has non-string value %#v", path, value)
	}
	return text
}

type memorySecretStore struct {
	values map[string][]byte
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: map[string][]byte{}}
}

func (s *memorySecretStore) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := s.values[key]
	if !ok {
		return nil, secrets.ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (s *memorySecretStore) Set(_ context.Context, key string, value []byte) error {
	s.values[key] = append([]byte(nil), value...)
	return nil
}

func (s *memorySecretStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func (s *memorySecretStore) List(context.Context) ([]string, error) {
	keys := make([]string, 0, len(s.values))
	for key := range s.values {
		keys = append(keys, key)
	}
	return keys, nil
}

func TestRestoreRedactedConfigSecretsLeavesOmittedSectionsAbsent(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"adapters": []any{adapterDocument("qq-official", "qqofficial", map[string]any{
			"app_secret": "stored-app-secret",
		})},
	}
	// The caller is not configuring the QQ adapter at all. Restoring its secret
	// would build a qqofficial block holding only app_secret, which fails that
	// section's required fields on the very next validation.
	request := map[string]any{
		"adapters": []any{map[string]any{
			"id":   "qq-official",
			"type": "qqofficial",
		}},
	}

	restored := restoreRedactedConfigSecrets(request, current)
	entry, ok := lookupConfigPath(restored, []string{"adapters", "qq-official"})
	if !ok {
		t.Fatal("restored document lost the adapter the request sent")
	}
	if settings, present := entry.(map[string]any)["qqofficial"]; present {
		t.Fatalf("restored document gained an omitted section: %#v", settings)
	}
}

func TestSecretPathsResolvePerDocumentNotPerSchema(t *testing.T) {
	t.Parallel()

	// The schema states shapes; only a document says which entries exist. A
	// shape whose settings block this entry does not hold resolves to nothing,
	// so no path is produced for a field that cannot be there.
	document := map[string]any{
		"adapters": []any{adapterDocument("onebot11", "onebot11", map[string]any{
			"forward_ws": map[string]any{"access_token": "forward-secret"},
		})},
	}

	resolved := make([]string, 0)
	for _, path := range configSecretPathsIn(document) {
		resolved = append(resolved, strings.Join(path, "."))
	}
	want := []string{
		"adapters.onebot11.onebot11.forward_ws.access_token",
		"adapters.onebot11.onebot11.http_api.access_token",
		"adapters.onebot11.onebot11.reverse_ws.access_token",
		"adapters.onebot11.onebot11.webhook.access_token",
	}
	if !reflect.DeepEqual(resolved, want) {
		t.Fatalf("resolved paths = %#v, want %#v", resolved, want)
	}
}

func TestLookupAddressesCollectionEntriesByKey(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		"adapters": []any{
			map[string]any{"id": "first", "settings": map[string]any{"token": "one"}},
			map[string]any{"id": "second", "settings": map[string]any{"token": "two"}},
		},
	}
	// Addressing by identifier rather than position means reordering the list
	// does not move a secret.
	value, ok := lookupConfigPath(document, []string{"adapters", "second", "settings", "token"})
	if !ok || value != "two" {
		t.Fatalf("lookup = %v (ok %v), want the second entry's token", value, ok)
	}
	if _, ok := lookupConfigPath(document, []string{"adapters", "missing", "settings", "token"}); ok {
		t.Fatal("an absent entry was resolved")
	}

	setConfigPath(document, []string{"adapters", "first", "settings", "token"}, "replaced")
	value, _ = lookupConfigPath(document, []string{"adapters", "first", "settings", "token"})
	if value != "replaced" {
		t.Fatalf("set then lookup = %v, want replaced", value)
	}
	// Writing into an entry that does not exist must not invent one: it would
	// have no identity.
	setConfigPath(document, []string{"adapters", "ghost", "settings", "token"}, "x")
	if len(document["adapters"].([]any)) != 2 {
		t.Fatal("setConfigPath invented a collection entry")
	}
}

func mustOneBot(t *testing.T, cfg internalconfig.Config) internalconfig.OneBotConfig {
	t.Helper()
	_, settings, ok := cfg.PrimaryOneBot11()
	if !ok {
		t.Fatal("resolved config has no onebot11 adapter")
	}
	return settings
}

func TestMigrateConfigSecretKeysMovesSealedValuesToTheirNewKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemorySecretStore()
	sealAt := func(key, plaintext string) {
		t.Helper()
		sealed, err := secrets.SealString(ctx, store, plaintext)
		if err != nil {
			t.Fatalf("seal %s: %v", key, err)
		}
		if err := store.Set(ctx, key, sealed); err != nil {
			t.Fatalf("store %s: %v", key, err)
		}
	}
	// The sealed values are still under the keys the pre-migration paths named.
	sealAt("config.onebot.forward_ws.access_token", "forward-secret")
	sealAt("config.qq_official.app_secret", "qq-app-secret")

	document := map[string]any{
		"adapters": []any{
			adapterDocument(internalconfig.DefaultOneBot11AdapterID, "onebot11", map[string]any{
				"forward_ws": map[string]any{"access_token": configSecretReference(onebotSecretPath(internalconfig.DefaultOneBot11AdapterID, "forward_ws"))},
			}),
			adapterDocument(internalconfig.DefaultQQOfficialAdapterID, "qqofficial", map[string]any{
				"app_secret": "secret://adapters/qq-official/qqofficial/app_secret",
			}),
		},
	}

	if err := MigrateConfigSecretKeys(ctx, store, document); err != nil {
		t.Fatalf("MigrateConfigSecretKeys() error = %v", err)
	}

	forwardPath := onebotSecretPath(internalconfig.DefaultOneBot11AdapterID, "forward_ws")
	assertOpensTo(t, ctx, store, configSecretKey(forwardPath), "forward-secret")
	appSecretPath := []string{"adapters", internalconfig.DefaultQQOfficialAdapterID, "qqofficial", "app_secret"}
	assertOpensTo(t, ctx, store, configSecretKey(appSecretPath), "qq-app-secret")
	for _, legacy := range []string{"config.onebot.forward_ws.access_token", "config.qq_official.app_secret"} {
		if _, err := store.Get(ctx, legacy); !errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("legacy key %s still present: %v", legacy, err)
		}
	}

	// Running again finds every secret in place and changes nothing, so an
	// install that already migrated is not disturbed on each start.
	if err := MigrateConfigSecretKeys(ctx, store, document); err != nil {
		t.Fatalf("second MigrateConfigSecretKeys() error = %v", err)
	}
	assertOpensTo(t, ctx, store, configSecretKey(forwardPath), "forward-secret")
}

func TestMigrateConfigSecretKeysLeavesAddedInstancesAlone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemorySecretStore()
	sealed, err := secrets.SealString(ctx, store, "forward-secret")
	if err != nil {
		t.Fatalf("seal fixture secret: %v", err)
	}
	if err := store.Set(ctx, "config.onebot.forward_ws.access_token", sealed); err != nil {
		t.Fatalf("store fixture secret: %v", err)
	}

	// An instance the operator added never had a pre-migration location, so the
	// legacy secret must not be adopted into it.
	document := map[string]any{
		"adapters": []any{adapterDocument("second-bot", "onebot11", map[string]any{
			"forward_ws": map[string]any{"access_token": ""},
		})},
	}
	if err := MigrateConfigSecretKeys(ctx, store, document); err != nil {
		t.Fatalf("MigrateConfigSecretKeys() error = %v", err)
	}
	if _, err := store.Get(ctx, configSecretKey(onebotSecretPath("second-bot", "forward_ws"))); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("added instance adopted a legacy secret: %v", err)
	}
	if _, err := store.Get(ctx, "config.onebot.forward_ws.access_token"); err != nil {
		t.Fatalf("legacy secret was removed without a destination: %v", err)
	}
}

func assertOpensTo(t *testing.T, ctx context.Context, store *memorySecretStore, key, want string) {
	t.Helper()
	stored, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	opened, err := secrets.OpenString(ctx, store, stored)
	if err != nil {
		t.Fatalf("open %s: %v", key, err)
	}
	if opened != want {
		t.Fatalf("%s = %q, want %q", key, opened, want)
	}
}
