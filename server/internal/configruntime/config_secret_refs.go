package configruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

const configSecretReferencePrefix = "secret://"

func StoreConfigSecrets(ctx context.Context, store secrets.Store, document map[string]any) (map[string]any, error) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}
	if store == nil {
		return cloned, nil
	}

	for _, path := range configSecretPathsIn(document) {
		value, ok := lookupConfigPath(cloned, path)
		if !ok {
			continue
		}
		text := strings.TrimSpace(stringValue(value))
		key := configSecretKey(path)
		reference := configSecretReference(path)
		switch {
		case text == "":
			if err := store.Delete(ctx, key); err != nil {
				return nil, fmt.Errorf("delete config secret %s: %w", strings.Join(path, "."), err)
			}
			setConfigPath(cloned, path, "")
		case isConfigSecretReference(text):
			if text != reference {
				return nil, fmt.Errorf("config secret %s must use %s", strings.Join(path, "."), reference)
			}
			if err := verifyConfigSecretReference(ctx, store, path); err != nil {
				return nil, err
			}
			setConfigPath(cloned, path, reference)
		default:
			sealed, err := secrets.SealString(ctx, store, text)
			if err != nil {
				return nil, fmt.Errorf("seal config secret %s: %w", strings.Join(path, "."), err)
			}
			if err := store.Set(ctx, key, sealed); err != nil {
				return nil, fmt.Errorf("store config secret %s: %w", strings.Join(path, "."), err)
			}
			setConfigPath(cloned, path, reference)
		}
	}
	return cloned, nil
}

func ResolveConfigSecretRefs(ctx context.Context, store secrets.Store, cfg internalconfig.Config) (internalconfig.Config, error) {
	document := ConfigDocumentFromTyped(cfg)
	for _, path := range configSecretPathsIn(document) {
		value, ok := lookupConfigPath(document, path)
		if !ok {
			continue
		}
		resolved, err := resolveConfigSecretRef(ctx, store, stringValue(value), path)
		if err != nil {
			return internalconfig.Config{}, err
		}
		setConfigPath(document, path, resolved)
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return internalconfig.Config{}, fmt.Errorf("encode resolved config secrets: %w", err)
	}
	var resolved internalconfig.Config
	if err := json.Unmarshal(payload, &resolved); err != nil {
		return internalconfig.Config{}, fmt.Errorf("decode resolved config secrets: %w", err)
	}
	return resolved, nil
}

func ConfigSecretValues(cfg internalconfig.Config) []string {
	return configSecretValues(cfg)
}

func resolveConfigSecretRef(ctx context.Context, store secrets.Store, value string, path []string) (string, error) {
	text := strings.TrimSpace(value)
	if text == "" || !isConfigSecretReference(text) {
		return text, nil
	}
	if store == nil {
		return "", fmt.Errorf("config secret %s requires secret store", strings.Join(path, "."))
	}
	if text != configSecretReference(path) {
		return "", fmt.Errorf("config secret %s must use %s", strings.Join(path, "."), configSecretReference(path))
	}
	stored, err := store.Get(ctx, configSecretKey(path))
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return "", fmt.Errorf("config secret %s not found", strings.Join(path, "."))
		}
		return "", fmt.Errorf("read config secret %s: %w", strings.Join(path, "."), err)
	}
	opened, err := secrets.OpenString(ctx, store, stored)
	if err != nil {
		return "", fmt.Errorf("open config secret %s: %w", strings.Join(path, "."), err)
	}
	return opened, nil
}

func verifyConfigSecretReference(ctx context.Context, store secrets.Store, path []string) error {
	if _, err := resolveConfigSecretRef(ctx, store, configSecretReference(path), path); err != nil {
		return err
	}
	return nil
}

func isConfigSecretReference(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), configSecretReferencePrefix)
}

func configSecretKey(path []string) string {
	return internalconfig.SecretStoreKeyFor(path)
}

func configSecretReference(path []string) string {
	return internalconfig.SecretReferenceFor(path)
}

// MigrateConfigSecretKeys moves sealed values whose storage key changed with the
// adapters migration. Document migration rewrote the references in the config;
// the sealed values live in the store and have to follow, which needs the store
// and so happens at assembly rather than during migration.
//
// It is idempotent: a secret already stored under its current key is left alone,
// so running it on every start costs one lookup per secret and nothing else.
func MigrateConfigSecretKeys(ctx context.Context, store secrets.Store, document map[string]any) error {
	if store == nil {
		return nil
	}
	for _, path := range configSecretPathsIn(document) {
		key := configSecretKey(path)
		if _, err := store.Get(ctx, key); err == nil {
			continue
		} else if !errors.Is(err, secrets.ErrNotFound) {
			return fmt.Errorf("read config secret %s: %w", strings.Join(path, "."), err)
		}
		legacyPath, ok := internalconfig.LegacyConfigSecretPath(path)
		if !ok {
			continue
		}
		legacyKey := internalconfig.SecretStoreKeyFor(legacyPath)
		stored, err := store.Get(ctx, legacyKey)
		if errors.Is(err, secrets.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read config secret %s: %w", strings.Join(legacyPath, "."), err)
		}
		if err := store.Set(ctx, key, stored); err != nil {
			return fmt.Errorf("move config secret %s: %w", strings.Join(path, "."), err)
		}
		// Only drop the old copy once the new one is stored, so an interrupted
		// start never leaves the secret in neither place.
		if err := store.Delete(ctx, legacyKey); err != nil {
			return fmt.Errorf("delete migrated config secret %s: %w", strings.Join(legacyPath, "."), err)
		}
	}
	return nil
}
