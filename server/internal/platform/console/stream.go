package console

import (
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/pubsub"
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
	history      []Entry
	historyHead  int
	historyCount int
	historySize  int
	hub          pubsub.Hub[Entry]
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

	cloned := make([]Entry, state.historyCount)
	state.copyHistory(cloned)
	return cloned
}

func (s *Stream) Append(entry Entry) {
	if entry.PluginID == "" || entry.Stream == "" || entry.Text == "" {
		return
	}

	entry.Timestamp = entry.Timestamp.UTC()
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	entrySize := len(entry.Text)

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.ensurePluginLocked(entry.PluginID)
	for state.historyCount >= s.maxEntries || state.historySize+entrySize > s.maxBytes {
		if state.historyCount == 0 {
			break
		}
		state.historySize -= len(state.history[state.historyHead].Text)
		state.history[state.historyHead] = Entry{}
		state.historyHead = (state.historyHead + 1) % len(state.history)
		state.historyCount--
	}

	if state.historyCount == len(state.history) {
		history := make([]Entry, min(s.maxEntries, max(1, len(state.history)*2)))
		state.copyHistory(history)
		state.history, state.historyHead = history, 0
	}
	state.history[(state.historyHead+state.historyCount)%len(state.history)] = entry
	state.historyCount++
	state.historySize += entrySize

	state.hub.PublishReplace(entry)
}

// copyHistory keeps snapshots chronological even after the bounded buffer wraps.
// Callers hold the owning Stream's lock.
func (state *pluginStream) copyHistory(destination []Entry) {
	if state.historyCount == 0 {
		return
	}
	first := min(state.historyCount, len(state.history)-state.historyHead)
	copy(destination, state.history[state.historyHead:state.historyHead+first])
	copy(destination[first:], state.history[:state.historyCount-first])
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
