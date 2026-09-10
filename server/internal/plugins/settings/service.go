// Package settings owns plugin settings and credential side effects shared by
// authenticated management requests and plugin-local actions.
package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

var (
	ErrInvalidValues  = errors.New("invalid plugin settings or credentials")
	ErrUnavailable    = errors.New("plugin settings storage is unavailable")
	ErrPluginNotFound = errors.New("plugin is not registered")
)

type Repository interface {
	ReadAll(context.Context, string) (map[string]any, error)
	Write(context.Context, string, map[string]any) ([]string, error)
}

type Deps struct {
	Plugins         plugins.CatalogView
	Config          Repository
	Secrets         secrets.Store
	RefreshCommands func(context.Context, string, map[string]any) error
	Notify          func(context.Context, string, map[string]any, []string) error
}

type Service struct {
	mu      sync.Mutex
	deps    Deps
	pending map[string]pendingEffects
}

type pendingEffects struct {
	keys     []string
	commands bool
}

type Update struct {
	Values      map[string]any
	ChangedKeys []string
}

// ApplyError distinguishes committed settings from their pending runtime effects.
type ApplyError struct {
	Stage       string
	ChangedKeys []string
	Cause       error
}

func (e *ApplyError) Error() string { return "plugin settings committed; runtime application pending" }
func (e *ApplyError) Unwrap() error { return e.Cause }
func (e *ApplyError) Details() map[string]any {
	return map[string]any{"committed": true, "stage": e.Stage, "changed_keys": append([]string{}, e.ChangedKeys...)}
}

// A service may expose credentials only; configured settings storage requires
// both runtime effects so a missing dependency cannot silently report success.
func New(deps Deps) (*Service, error) {
	if deps.Plugins == nil || (deps.Config == nil && deps.Secrets == nil) {
		return nil, ErrUnavailable
	}
	if deps.Config != nil && (deps.RefreshCommands == nil || deps.Notify == nil) {
		return nil, errors.New("plugin settings runtime effects are required")
	}
	return &Service{deps: deps, pending: make(map[string]pendingEffects)}, nil
}

func (s *Service) Read(ctx context.Context, pluginID string) (map[string]any, error) {
	if s == nil || s.deps.Config == nil {
		return nil, ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(ctx, pluginID)
}

func (s *Service) read(ctx context.Context, pluginID string) (map[string]any, error) {
	snapshot, ok := s.deps.Plugins.Get(pluginID)
	if !ok {
		return nil, ErrPluginNotFound
	}
	persisted, err := s.deps.Config.ReadAll(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	return pluginstore.MergeValues(snapshot.DefaultConfig, persisted), nil
}

func (s *Service) Write(ctx context.Context, pluginID string, values map[string]any) (Update, error) {
	if s == nil || s.deps.Config == nil {
		return Update{}, ErrUnavailable
	}
	if values == nil {
		return Update{}, ErrInvalidValues
	}
	for key := range values {
		if key == "" {
			return Update{}, ErrInvalidValues
		}
	}
	// Own a JSON snapshot before taking the write lock. Numeric Go representations
	// and object insertion order do not create false effective-value changes.
	raw, err := json.Marshal(values)
	if err != nil {
		return Update{}, ErrInvalidValues
	}
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		return Update{}, ErrInvalidValues
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before, err := s.read(ctx, pluginID)
	if err != nil {
		return Update{}, err
	}
	changed := make([]string, 0, len(input))
	for key, value := range input {
		previous, exists := before[key]
		if !exists || !equalJSON(previous, value) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	if _, err := s.deps.Config.Write(ctx, pluginID, input); err != nil {
		return Update{}, err
	}
	after := pluginstore.MergeValues(before, input)
	pending := s.pending[pluginID]
	if len(changed) > 0 {
		pending.keys = mergeKeys(pending.keys, changed)
		pending.commands = true
		s.pending[pluginID] = pending
	}
	result := Update{Values: after, ChangedKeys: append([]string{}, pending.keys...)}
	if len(pending.keys) == 0 {
		return result, nil
	}
	if pending.commands {
		if err := s.deps.RefreshCommands(ctx, pluginID, pluginstore.MergeValues(after, nil)); err != nil {
			return result, &ApplyError{Stage: "commands", ChangedKeys: append([]string{}, pending.keys...), Cause: err}
		}
		pending.commands = false
		s.pending[pluginID] = pending
	}
	if err := s.deps.Notify(ctx, pluginID, pluginstore.MergeValues(after, nil), append([]string{}, pending.keys...)); err != nil {
		return result, &ApplyError{Stage: "notification", ChangedKeys: append([]string{}, pending.keys...), Cause: err}
	}
	delete(s.pending, pluginID)
	return result, nil
}

func equalJSON(left, right any) bool {
	a, errA := json.Marshal(left)
	b, errB := json.Marshal(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func mergeKeys(left, right []string) []string {
	keys := make(map[string]struct{}, len(left)+len(right))
	for _, key := range left {
		keys[key] = struct{}{}
	}
	for _, key := range right {
		keys[key] = struct{}{}
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

// Activate serializes publication with settings writes and resumes changes made during initialization.
func (s *Service) Activate(ctx context.Context, pluginID string, initialized map[string]any, publish func() error) error {
	if s == nil || s.deps.Config == nil || publish == nil {
		return ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.read(ctx, pluginID)
	if err != nil {
		return err
	}
	if err := s.deps.RefreshCommands(ctx, pluginID, pluginstore.MergeValues(current, nil)); err != nil {
		return err
	}
	if err := publish(); err != nil {
		return err
	}
	changed := make([]string, 0)
	for key, value := range current {
		previous, exists := initialized[key]
		if !exists || !equalJSON(value, previous) {
			changed = append(changed, key)
		}
	}
	for key := range initialized {
		if _, exists := current[key]; !exists {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	if len(changed) > 0 {
		s.pending[pluginID] = pendingEffects{keys: changed}
		if err := s.deps.Notify(ctx, pluginID, pluginstore.MergeValues(current, nil), append([]string{}, changed...)); err != nil {
			return &ApplyError{Stage: "notification", ChangedKeys: append([]string{}, changed...), Cause: err}
		}
	}
	delete(s.pending, pluginID)
	return nil
}
