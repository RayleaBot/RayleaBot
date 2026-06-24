package configruntime

import (
	"context"
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

	for _, path := range secretConfigPaths {
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
	var err error
	if cfg.OneBot.ForwardWS.AccessToken, err = resolveConfigSecretRef(ctx, store, cfg.OneBot.ForwardWS.AccessToken, []string{"onebot", "forward_ws", "access_token"}); err != nil {
		return internalconfig.Config{}, err
	}
	if cfg.OneBot.HTTPAPI.AccessToken, err = resolveConfigSecretRef(ctx, store, cfg.OneBot.HTTPAPI.AccessToken, []string{"onebot", "http_api", "access_token"}); err != nil {
		return internalconfig.Config{}, err
	}
	if cfg.OneBot.ReverseWS.AccessToken, err = resolveConfigSecretRef(ctx, store, cfg.OneBot.ReverseWS.AccessToken, []string{"onebot", "reverse_ws", "access_token"}); err != nil {
		return internalconfig.Config{}, err
	}
	if cfg.OneBot.Webhook.AccessToken, err = resolveConfigSecretRef(ctx, store, cfg.OneBot.Webhook.AccessToken, []string{"onebot", "webhook", "access_token"}); err != nil {
		return internalconfig.Config{}, err
	}
	return cfg, nil
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
	return "config." + strings.Join(path, ".")
}

func configSecretReference(path []string) string {
	return configSecretReferencePrefix + strings.Join(path, "/")
}
