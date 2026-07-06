package console

import (
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/pubsub"
)

const (
	defaultHistoryEntries = 1000
	defaultHistoryBytes   = 2 * 1024 * 1024
)

type Entry struct {
	PluginID  string    `json:"plugin_id"`
	Stream    string    `json:"stream"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

type pluginStream struct {
	history     []Entry
	historySize int
	hub         pubsub.Hub[Entry]
}

type Stream struct {
	mu         sync.RWMutex
	maxEntries int
	maxBytes   int
	plugins    map[string]*pluginStream
}

func NewStream(maxEntries, maxBytes int) *Stream {
	if maxEntries <= 0 {
		maxEntries = defaultHistoryEntries
	}
	if maxBytes <= 0 {
		maxBytes = defaultHistoryBytes
	}

	return &Stream{
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
		plugins:    map[string]*pluginStream{},
	}
}

func (s *Stream) Snapshot(pluginID string) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.plugins[pluginID]
	if !ok {
		return nil
	}

	cloned := make([]Entry, len(state.history))
	copy(cloned, state.history)
	return cloned
}

func (s *Stream) Append(entry Entry) {
	if s == nil || entry.PluginID == "" || entry.Stream == "" || entry.Text == "" {
		return
	}

	entry.Timestamp = entry.Timestamp.UTC()
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	entrySize := len([]byte(entry.Text))
	if entrySize <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.ensurePluginLocked(entry.PluginID)
	for len(state.history) >= s.maxEntries || state.historySize+entrySize > s.maxBytes {
		if len(state.history) == 0 {
			break
		}
		removed := state.history[0]
		state.history = append([]Entry(nil), state.history[1:]...)
		state.historySize -= len([]byte(removed.Text))
	}

	state.history = append(state.history, entry)
	state.historySize += entrySize

	state.hub.PublishReplace(entry)
}

func (s *Stream) Subscribe(pluginID string, buffer int) (<-chan Entry, func()) {
	s.mu.Lock()
	state := s.ensurePluginLocked(pluginID)
	s.mu.Unlock()

	return state.hub.Subscribe(buffer)
}

func (s *Stream) SubscriberCount(pluginID string) int {
	s.mu.RLock()
	state, ok := s.plugins[pluginID]
	s.mu.RUnlock()
	if !ok {
		return 0
	}

	return state.hub.SubscriberCount()
}

func (s *Stream) ensurePluginLocked(pluginID string) *pluginStream {
	state, ok := s.plugins[pluginID]
	if ok {
		return state
	}

	state = &pluginStream{}
	s.plugins[pluginID] = state
	return state
}
