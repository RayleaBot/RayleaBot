// Package messagestats owns message counters and connection/run history.
package messagestats

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const flushInterval = 30 * time.Second

type Options struct {
	Logger        *slog.Logger
	Store         *storage.Store
	CurrentConfig func() config.Config
	Timezone      string
	Now           func() time.Time
}

type hourKey struct {
	start   int64
	adapter string
}
type offlineKey struct {
	run     int64
	adapter string
	start   int64
}
type adapterState struct {
	enabled, connected bool
	since              time.Time
}

type Service struct {
	loopCancel context.CancelFunc
	loopDone   chan struct{}
	// ioMu serializes queries and flushes through commit or merge-back.
	// Always take ioMu before mu; mu protects memory only, never database I/O.
	ioMu              sync.Mutex
	mu                sync.Mutex
	store             *storage.Store
	now               func() time.Time
	config            func() config.Config
	location          *time.Location
	tracking, stopped time.Time
	runID             int64
	pending           map[hourKey]Counts
	metadata          map[string]sqlcgen.UpsertMessageStatsAdapterParams
	states            map[string]adapterState
	offline           map[offlineKey]sql.NullInt64
}

func New(ctx context.Context, options Options) (*Service, error) {
	if options.Store == nil || options.Store.Read == nil || options.Store.Write == nil {
		return nil, errors.New("message statistics store is required")
	}
	location, err := time.LoadLocation(options.Timezone)
	if err != nil {
		return nil, err
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.CurrentConfig == nil {
		options.CurrentConfig = func() config.Config { return config.Config{} }
	}
	s := &Service{store: options.Store, now: options.Now, config: options.CurrentConfig, location: location,
		pending: make(map[hourKey]Counts), metadata: make(map[string]sqlcgen.UpsertMessageStatsAdapterParams),
		states: make(map[string]adapterState), offline: make(map[offlineKey]sql.NullInt64)}
	now := s.timeNow()
	err = storage.WithTx(ctx, s.store.Write, nil, func(tx *sql.Tx) error {
		q := sqlcgen.New(tx)
		if err := q.StartMessageTracking(ctx, now.UnixMilli()); err != nil {
			return err
		}
		started, err := q.GetMessageTracking(ctx)
		if err != nil {
			return err
		}
		s.tracking = time.UnixMilli(started).UTC()
		if err := q.RecoverMessageStatsOffline(ctx); err != nil {
			return err
		}
		if err := q.RecoverMessageStatsRuns(ctx); err != nil {
			return err
		}
		s.runID, err = q.StartMessageStatsRun(ctx, sqlcgen.StartMessageStatsRunParams{StartedAtMs: now.UnixMilli(), LastAliveAtMs: now.UnixMilli()})
		return err
	})
	if err != nil {
		return nil, err
	}
	for _, a := range s.config().Adapters {
		state := adapterState{enabled: a.Enabled}
		if a.Enabled {
			state.since = now.Add(time.Minute)
		}
		s.states[a.ID] = state
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	loopCtx, cancel := context.WithCancel(context.Background())
	s.loopCancel, s.loopDone = cancel, make(chan struct{})
	go func() { defer close(s.loopDone); s.run(loopCtx, logger) }()
	return s, nil
}

func (s *Service) timeNow() time.Time { return s.now().UTC().Truncate(time.Millisecond) }

// Received is called at host ingress, before any routing or governance.
func (s *Service) Received(event chatevent.NormalizedEvent) {
	if event.EventType != "message.group" && event.EventType != "message.private" {
		return
	}
	family := chatevent.EventFamily(event.Kind)
	if family != chatevent.FamilyMessage && family != chatevent.FamilyMessageText {
		return
	}
	s.count(event.SourceAdapter, event.SourceProtocol, true)
}

// Sent only observes platform-confirmed successful logical sends.
func (s *Service) Sent(adapterID, protocol string) { s.count(adapterID, protocol, false) }

func (s *Service) count(adapterID, protocol string, received bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped.IsZero() {
		return
	}
	now := s.timeNow()
	key := hourKey{now.Truncate(time.Hour).Unix(), adapterID}
	count := s.pending[key]
	meta := s.metadata[adapterID]
	meta.AdapterID, meta.Protocol = adapterID, protocol
	if received {
		count.Received++
		if !meta.LastReceivedAtMs.Valid || now.UnixMilli() > meta.LastReceivedAtMs.Int64 {
			meta.LastReceivedAtMs = sql.NullInt64{Int64: now.UnixMilli(), Valid: true}
		}
	} else {
		count.Sent++
	}
	s.pending[key], s.metadata[adapterID] = count, meta
}

type Adapter struct {
	ID                 string
	Enabled, Connected bool
}

// ObserveAdapters consumes the authoritative adapter snapshot synchronously.
// Omitted adapters have been removed, so their open intervals end here.
func (s *Service) ObserveAdapters(adapters []Adapter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped.IsZero() {
		return
	}
	now := s.timeNow()
	seen := make(map[string]bool, len(adapters))
	for _, a := range adapters {
		seen[a.ID] = true
		old, exists := s.states[a.ID]
		next := old
		next.enabled, next.connected = a.Enabled, a.Connected
		switch {
		case !a.Enabled || a.Connected:
			s.endOffline(a.ID, old, now)
			next.since = time.Time{}
		case !exists || !old.enabled:
			next.since = now.Add(time.Minute)
		case old.connected:
			next.since = now
		}
		s.states[a.ID] = next
	}
	for id, state := range s.states {
		if !seen[id] {
			s.endOffline(id, state, now)
			delete(s.states, id)
		}
	}
}

// ReloadAdapter gives a changed connection a new 60 second startup grace.
func (s *Service) ReloadAdapter(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped.IsZero() {
		return
	}
	now := s.timeNow()
	old := s.states[id]
	s.endOffline(id, old, now)
	old.connected = false
	if old.enabled {
		old.since = now.Add(time.Minute)
	} else {
		old.since = time.Time{}
	}
	s.states[id] = old
}

func (s *Service) endOffline(id string, state adapterState, now time.Time) {
	if state.since.IsZero() || now.Sub(state.since) < 30*time.Second {
		return
	}
	s.offline[offlineKey{s.runID, id, state.since.UnixMilli()}] = sql.NullInt64{Int64: now.UnixMilli(), Valid: true}
}

func (s *Service) openOffline(now time.Time) map[offlineKey]sql.NullInt64 {
	result := make(map[offlineKey]sql.NullInt64, len(s.offline)+len(s.states))
	for k, v := range s.offline {
		result[k] = v
	}
	for id, state := range s.states {
		if !state.since.IsZero() && now.Sub(state.since) >= 30*time.Second {
			result[offlineKey{s.runID, id, state.since.UnixMilli()}] = sql.NullInt64{}
		}
	}
	return result
}

func (s *Service) run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := s.Flush(flushCtx)
			cancel()
			if err != nil && ctx.Err() == nil {
				logger.Error("消息统计保存失败", "component", "message_stats", "err", err)
			}
		}
	}
}

func (s *Service) Flush(ctx context.Context) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	return s.flush(ctx)
}

func (s *Service) Stop(ctx context.Context) error {
	// Keep heartbeats running through event/plugin drain; only final persistence
	// ends this worker. Cancellation and the final transaction share its budget.
	s.loopCancel()
	select {
	case <-s.loopDone:
	case <-ctx.Done():
		return ctx.Err()
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.Lock()
	if s.stopped.IsZero() {
		s.stopped = s.timeNow()
		for id, state := range s.states {
			s.endOffline(id, state, s.stopped)
		}
		clear(s.states)
	}
	s.mu.Unlock()
	return s.flush(ctx)
}

// flush requires ioMu until the batch is committed or restored.
func (s *Service) flush(ctx context.Context) error {
	s.mu.Lock()
	now, stopped := s.timeNow(), s.stopped
	if !stopped.IsZero() {
		now = stopped
	}
	intervals := s.openOffline(now)
	pending, metadata, offline := s.pending, s.metadata, s.offline
	s.pending = make(map[hourKey]Counts)
	s.metadata = make(map[string]sqlcgen.UpsertMessageStatsAdapterParams)
	s.offline = make(map[offlineKey]sql.NullInt64)
	s.mu.Unlock()

	err := storage.WithTx(ctx, s.store.Write, nil, func(tx *sql.Tx) error {
		q := sqlcgen.New(tx)
		for _, meta := range metadata {
			if err := q.UpsertMessageStatsAdapter(ctx, meta); err != nil {
				return err
			}
		}
		for key, count := range pending {
			if err := q.AddMessageStatsHour(ctx, sqlcgen.AddMessageStatsHourParams{HourStart: key.start, AdapterID: key.adapter, Received: count.Received, Sent: count.Sent}); err != nil {
				return err
			}
		}
		for key, end := range intervals {
			if err := q.UpsertMessageStatsOffline(ctx, sqlcgen.UpsertMessageStatsOfflineParams{RunID: key.run, AdapterID: key.adapter, StartedAtMs: key.start, EndedAtMs: end}); err != nil {
				return err
			}
		}
		return q.UpdateMessageStatsRun(ctx, sqlcgen.UpdateMessageStatsRunParams{ID: s.runID, LastAliveAtMs: now.UnixMilli(), StoppedAtMs: sql.NullInt64{Int64: stopped.UnixMilli(), Valid: !stopped.IsZero()}})
	})
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		for key, count := range pending {
			total := s.pending[key]
			total.add(count)
			s.pending[key] = total
		}
		for id, meta := range metadata {
			current, exists := s.metadata[id]
			if !exists {
				current = meta
			} else if meta.LastReceivedAtMs.Valid && (!current.LastReceivedAtMs.Valid || meta.LastReceivedAtMs.Int64 > current.LastReceivedAtMs.Int64) {
				current.LastReceivedAtMs = meta.LastReceivedAtMs
			}
			s.metadata[id] = current
		}
		for key, end := range offline {
			if _, exists := s.offline[key]; !exists {
				s.offline[key] = end
			}
		}
	}
	return err
}
