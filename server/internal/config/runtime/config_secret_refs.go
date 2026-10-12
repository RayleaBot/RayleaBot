package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
)

const configSecretReferencePrefix = "secret://"

func removedConfigSecretKeys(previous, next map[string]any) []string {
	previousIndex := configDocumentIndex{document: previous}
	nextIndex := configDocumentIndex{document: next}
	var keys []string
	for _, path := range previousIndex.secretPaths() {
		if _, exists := previousIndex.lookup(path); !exists {
			continue
		}
		if _, exists := nextIndex.lookup(path); !exists {
			keys = append(keys, configSecretKey(path))
		}
	}
	return keys
}

func (s *Service) deleteRemovedConfigSecrets(ctx context.Context, keys []string) {
	if s.secrets == nil || len(keys) == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	for _, key := range keys {
		if err := s.secrets.Delete(cleanupCtx, key); err != nil && s.logger != nil {
			s.logger.Warn("配置已保存，但旧密钥清理失败",
				"component", "config",
				"key", key,
				"err", err.Error(),
			)
		}
	}
}

func StoreConfigSecrets(ctx context.Context, store secrets.Store, document map[string]any) (map[string]any, error) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}
	if store == nil {
		return cloned, nil
	}

	index := configDocumentIndex{document: cloned}
	for _, path := range configSecretPathsIn(document) {
		value, ok := index.lookup(path)
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
			index.set(path, "")
		case isConfigSecretReference(text):
			if text != reference {
				return nil, fmt.Errorf("config secret %s must use %s", strings.Join(path, "."), reference)
			}
			if err := verifyConfigSecretReference(ctx, store, path); err != nil {
				return nil, err
			}
			index.set(path, reference)
		default:
			if err := store.Set(ctx, key, []byte(text)); err != nil {
				return nil, fmt.Errorf("store config secret %s: %w", strings.Join(path, "."), err)
			}
			index.set(path, reference)
		}
	}
	return cloned, nil
}

func ResolveConfigSecretRefs(ctx context.Context, store secrets.Store, cfg internalconfig.Config) (internalconfig.Config, error) {
	document := ConfigDocumentFromTyped(cfg)
	index := configDocumentIndex{document: document}
	for _, path := range index.secretPaths() {
		value, ok := index.lookup(path)
		if !ok {
			continue
		}
		resolved, err := resolveConfigSecretRef(ctx, store, stringValue(value), path)
		if err != nil {
			return internalconfig.Config{}, err
		}
		index.set(path, resolved)
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
	return string(stored), nil
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
