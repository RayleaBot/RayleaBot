package configruntime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type secretValue struct {
	value   []byte
	present bool
}

// stagedSecrets resolves and validates the complete new configuration before
// changing persistent credentials. The old values support compensation if the
// secret store or the subsequent user.yaml replacement fails.
type stagedSecrets struct {
	base    secrets.Store
	before  map[string]secretValue
	pending map[string]secretValue
}

func newStagedSecrets(base secrets.Store) *stagedSecrets {
	return &stagedSecrets{base: base, before: make(map[string]secretValue), pending: make(map[string]secretValue)}
}

func (s *stagedSecrets) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if value, ok := s.pending[key]; ok {
		if !value.present {
			return nil, secrets.ErrNotFound
		}
		return slices.Clone(value.value), nil
	}
	return s.base.Get(ctx, key)
}

func (s *stagedSecrets) capture(ctx context.Context, key string) error {
	if _, ok := s.before[key]; ok {
		return nil
	}
	value, err := s.base.Get(ctx, key)
	if err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return err
	}
	s.before[key] = secretValue{value: slices.Clone(value), present: err == nil}
	return nil
}

func (s *stagedSecrets) Set(ctx context.Context, key string, value []byte) error {
	if err := s.capture(ctx, key); err != nil {
		return err
	}
	s.pending[key] = secretValue{value: slices.Clone(value), present: true}
	return nil
}

func (s *stagedSecrets) Delete(ctx context.Context, key string) error {
	if err := s.capture(ctx, key); err != nil {
		return err
	}
	s.pending[key] = secretValue{}
	return nil
}

func (s *stagedSecrets) List(ctx context.Context) ([]string, error) {
	keys, err := s.base.List(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(keys)+len(s.pending))
	for _, key := range keys {
		set[key] = true
	}
	for key, value := range s.pending {
		set[key] = value.present
	}
	keys = keys[:0]
	for key, present := range set {
		if present {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys, nil
}

func (s *stagedSecrets) persist(ctx context.Context, saveDocument func() error) error {
	keys := make([]string, 0, len(s.pending))
	for key := range s.pending {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	written := make([]string, 0, len(keys))
	for _, key := range keys {
		written = append(written, key)
		if err := s.write(ctx, key, s.pending[key]); err != nil {
			return errors.Join(fmt.Errorf("persist config credentials: %w", err), s.rollback(ctx, written))
		}
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(err, s.rollback(ctx, written))
	}
	if err := saveDocument(); err != nil {
		return errors.Join(err, s.rollback(ctx, written))
	}
	return nil
}

func (s *stagedSecrets) write(ctx context.Context, key string, value secretValue) error {
	if value.present {
		return s.base.Set(ctx, key, value.value)
	}
	return s.base.Delete(ctx, key)
}

func (s *stagedSecrets) rollback(ctx context.Context, keys []string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var failures []error
	for index := len(keys) - 1; index >= 0; index-- {
		if err := s.write(cleanupCtx, keys[index], s.before[keys[index]]); err != nil {
			failures = append(failures, fmt.Errorf("restore config credential: %w", err))
		}
	}
	return errors.Join(failures...)
}
