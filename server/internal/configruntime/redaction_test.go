package configruntime

import (
	"context"
	"errors"
	"reflect"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

func TestSanitizeConfigDocumentRedactsOneBotTransportTokens(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		"onebot": map[string]any{
			"forward_ws": map[string]any{"access_token": "forward-secret"},
			"http_api":   map[string]any{"access_token": "http-secret"},
			"reverse_ws": map[string]any{"access_token": "reverse-secret"},
			"webhook":    map[string]any{"access_token": "webhook-secret"},
		},
	}

	redacted, fields := sanitizeConfigDocument(document)
	wantFields := []string{
		"onebot.forward_ws.access_token",
		"onebot.http_api.access_token",
		"onebot.reverse_ws.access_token",
		"onebot.webhook.access_token",
	}
	if !reflect.DeepEqual(fields, wantFields) {
		t.Fatalf("redacted fields = %#v, want %#v", fields, wantFields)
	}
	for _, path := range secretConfigPaths {
		if got := stringAtPath(t, redacted, path); got != redactedConfigValue {
			t.Fatalf("%s = %q, want redacted marker", path[len(path)-2], got)
		}
	}
	if got := stringAtPath(t, document, []string{"onebot", "forward_ws", "access_token"}); got != "forward-secret" {
		t.Fatalf("original document was mutated: %q", got)
	}
}

func TestRestoreRedactedConfigSecretsRetainsReplacesAndClears(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"onebot": map[string]any{
			"forward_ws": map[string]any{"access_token": "old-forward"},
			"http_api":   map[string]any{"access_token": "old-http"},
			"reverse_ws": map[string]any{"access_token": "old-reverse"},
			"webhook":    map[string]any{"access_token": "old-webhook"},
		},
	}
	request := map[string]any{
		"onebot": map[string]any{
			"forward_ws": map[string]any{"access_token": "new-forward"},
			"http_api":   map[string]any{"access_token": ""},
			"reverse_ws": map[string]any{"access_token": redactedConfigValue},
		},
	}

	restored := restoreRedactedConfigSecrets(request, current)
	assertStringAtPath(t, restored, []string{"onebot", "forward_ws", "access_token"}, "new-forward")
	assertStringAtPath(t, restored, []string{"onebot", "http_api", "access_token"}, "")
	assertStringAtPath(t, restored, []string{"onebot", "reverse_ws", "access_token"}, "old-reverse")
	assertStringAtPath(t, restored, []string{"onebot", "webhook", "access_token"}, "old-webhook")
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
	next := internalconfig.Config{
		OneBot: internalconfig.OneBotConfig{
			ForwardWS: internalconfig.OneBotTransportConfig{AccessToken: "forward-secret"},
			HTTPAPI:   internalconfig.OneBotTransportConfig{AccessToken: "http-secret"},
			ReverseWS: internalconfig.OneBotTransportConfig{AccessToken: "reverse-secret"},
			Webhook:   internalconfig.OneBotTransportConfig{AccessToken: "webhook-secret"},
		},
	}

	service.ApplyHotReloadableFields(next)
	want := []string{"forward-secret", "http-secret", "reverse-secret", "webhook-secret"}
	if !reflect.DeepEqual(added, want) {
		t.Fatalf("added redaction values = %#v, want %#v", added, want)
	}
}

func TestStoreConfigSecretsSealsOneBotTransportTokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemorySecretStore()
	document := map[string]any{
		"onebot": map[string]any{
			"forward_ws": map[string]any{"access_token": "forward-secret"},
			"http_api":   map[string]any{"access_token": "http-secret"},
			"reverse_ws": map[string]any{"access_token": "reverse-secret"},
			"webhook":    map[string]any{"access_token": "webhook-secret"},
		},
	}

	stored, err := StoreConfigSecrets(ctx, store, document)
	if err != nil {
		t.Fatalf("store config secrets: %v", err)
	}
	for _, path := range secretConfigPaths {
		if got := stringAtPath(t, stored, path); got != configSecretReference(path) {
			t.Fatalf("%v = %q, want %q", path, got, configSecretReference(path))
		}
		if got := stringAtPath(t, document, path); got == configSecretReference(path) {
			t.Fatalf("original document was mutated at %v", path)
		}
	}

	storedForward, err := store.Get(ctx, configSecretKey([]string{"onebot", "forward_ws", "access_token"}))
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

	resolved, err := ResolveConfigSecretRefs(ctx, store, internalconfig.Config{
		OneBot: internalconfig.OneBotConfig{
			ForwardWS: internalconfig.OneBotTransportConfig{AccessToken: configSecretReference([]string{"onebot", "forward_ws", "access_token"})},
			HTTPAPI:   internalconfig.OneBotTransportConfig{AccessToken: configSecretReference([]string{"onebot", "http_api", "access_token"})},
			ReverseWS: internalconfig.OneBotTransportConfig{AccessToken: configSecretReference([]string{"onebot", "reverse_ws", "access_token"})},
			Webhook:   internalconfig.OneBotTransportConfig{AccessToken: configSecretReference([]string{"onebot", "webhook", "access_token"})},
		},
	})
	if err != nil {
		t.Fatalf("resolve config secret refs: %v", err)
	}
	want := internalconfig.OneBotConfig{
		ForwardWS: internalconfig.OneBotTransportConfig{AccessToken: "forward-secret"},
		HTTPAPI:   internalconfig.OneBotTransportConfig{AccessToken: "http-secret"},
		ReverseWS: internalconfig.OneBotTransportConfig{AccessToken: "reverse-secret"},
		Webhook:   internalconfig.OneBotTransportConfig{AccessToken: "webhook-secret"},
	}
	if !reflect.DeepEqual(resolved.OneBot, want) {
		t.Fatalf("resolved onebot = %#v, want %#v", resolved.OneBot, want)
	}
}

func TestStoreConfigSecretsDeletesClearedToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemorySecretStore()
	path := []string{"onebot", "forward_ws", "access_token"}
	sealed, err := secrets.SealString(ctx, store, "forward-secret")
	if err != nil {
		t.Fatalf("seal fixture secret: %v", err)
	}
	if err := store.Set(ctx, configSecretKey(path), sealed); err != nil {
		t.Fatalf("store fixture secret: %v", err)
	}

	_, err = StoreConfigSecrets(ctx, store, map[string]any{
		"onebot": map[string]any{
			"forward_ws": map[string]any{"access_token": ""},
		},
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
		OneBot: internalconfig.OneBotConfig{
			ForwardWS: internalconfig.OneBotTransportConfig{AccessToken: "secret://onebot/reverse_ws/access_token"},
		},
	})
	if err == nil {
		t.Fatal("expected wrong secret reference to fail")
	}
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
