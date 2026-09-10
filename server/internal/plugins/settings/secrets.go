package settings

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
)

var secretKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$`)

func ValidSecretKey(key string) bool { return secretKeyPattern.MatchString(key) }

func secretStorageKey(pluginID, key string) string { return "plugin:" + pluginID + ":secret:" + key }

type SecretUpdate struct {
	ChangedKeys []string
	Configured  map[string]bool
}

func (s *Service) ReadSecret(ctx context.Context, pluginID, key string) (string, bool, error) {
	if s == nil || s.deps.Secrets == nil {
		return "", false, ErrUnavailable
	}
	if !ValidSecretKey(key) {
		return "", false, ErrInvalidValues
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.deps.Plugins.Get(pluginID); !ok {
		return "", false, ErrPluginNotFound
	}
	return s.readSecret(ctx, pluginID, key)
}

func (s *Service) readSecret(ctx context.Context, pluginID, key string) (string, bool, error) {
	value, err := s.deps.Secrets.Get(ctx, secretStorageKey(pluginID, key))
	if errors.Is(err, secrets.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	plaintext, err := secrets.OpenString(ctx, s.deps.Secrets, value)
	return plaintext, err == nil, err
}

func (s *Service) SecretStatus(ctx context.Context, pluginID string) (map[string]bool, error) {
	if s == nil || s.deps.Secrets == nil {
		return nil, ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.secretStatus(ctx, pluginID)
}

func (s *Service) secretStatus(ctx context.Context, pluginID string) (map[string]bool, error) {
	if _, ok := s.deps.Plugins.Get(pluginID); !ok {
		return nil, ErrPluginNotFound
	}
	keys, err := s.deps.Secrets.List(ctx)
	if err != nil {
		return nil, err
	}
	prefix := secretStorageKey(pluginID, "")
	configured := make(map[string]bool)
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			configured[strings.TrimPrefix(key, prefix)] = true
		}
	}
	return configured, nil
}

func (s *Service) SetSecrets(ctx context.Context, pluginID string, values map[string]string) (SecretUpdate, error) {
	if s == nil {
		return SecretUpdate{}, ErrUnavailable
	}
	store, ok := s.deps.Secrets.(secrets.BatchStore)
	if !ok {
		return SecretUpdate{}, ErrUnavailable
	}
	if len(values) == 0 {
		return SecretUpdate{}, ErrInvalidValues
	}
	for key, value := range values {
		if !ValidSecretKey(key) || value == "" {
			return SecretUpdate{}, ErrInvalidValues
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	configured, err := s.secretStatus(ctx, pluginID)
	if err != nil {
		return SecretUpdate{}, err
	}
	sealed := make(map[string][]byte)
	changed := make([]string, 0, len(values))
	for key, value := range values {
		previous, exists, err := s.readSecret(ctx, pluginID, key)
		if err != nil {
			return SecretUpdate{}, err
		}
		if exists && previous == value {
			continue
		}
		data, err := secrets.SealString(ctx, store, value)
		if err != nil {
			return SecretUpdate{}, err
		}
		sealed[secretStorageKey(pluginID, key)] = data
		changed = append(changed, key)
	}
	if len(sealed) > 0 {
		if err := store.Apply(ctx, sealed, nil); err != nil {
			return SecretUpdate{}, err
		}
	}
	for _, key := range changed {
		configured[key] = true
	}
	sort.Strings(changed)
	return SecretUpdate{ChangedKeys: changed, Configured: configured}, nil
}

func (s *Service) DeleteSecrets(ctx context.Context, pluginID string, keys []string) (SecretUpdate, error) {
	if s == nil {
		return SecretUpdate{}, ErrUnavailable
	}
	store, ok := s.deps.Secrets.(secrets.BatchStore)
	if !ok {
		return SecretUpdate{}, ErrUnavailable
	}
	if len(keys) == 0 {
		return SecretUpdate{}, ErrInvalidValues
	}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if !ValidSecretKey(key) || seen[key] {
			return SecretUpdate{}, ErrInvalidValues
		}
		seen[key] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	configured, err := s.secretStatus(ctx, pluginID)
	if err != nil {
		return SecretUpdate{}, err
	}
	deleted := make([]string, 0, len(keys))
	changed := make([]string, 0, len(keys))
	for _, key := range keys {
		if configured[key] {
			deleted = append(deleted, secretStorageKey(pluginID, key))
			changed = append(changed, key)
		}
		configured[key] = false
	}
	if len(deleted) > 0 {
		if err := store.Apply(ctx, nil, deleted); err != nil {
			return SecretUpdate{}, err
		}
	}
	sort.Strings(changed)
	return SecretUpdate{ChangedKeys: changed, Configured: configured}, nil
}
