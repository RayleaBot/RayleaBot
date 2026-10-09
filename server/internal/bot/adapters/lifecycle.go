package adapters

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

func (s *Service) Start(ctx context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopped {
		return ErrStopped
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.runCtx = ctx
	snapshot := s.registry.Snapshot()
	for _, shell := range snapshot.oneBot11 {
		shell.Start(ctx)
	}
	for _, client := range snapshot.qqOfficial {
		client.Start(ctx)
	}
	return ctx.Err()
}

func (s *Service) Stop(ctx context.Context) error {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	s.lifecycleMu.Lock()
	s.stopped = true
	s.lifecycleMu.Unlock()
	snapshot := s.registry.Snapshot()
	entries := make([]stopEntry, 0, len(snapshot.oneBot11)+len(snapshot.qqOfficial))
	for id, shell := range snapshot.oneBot11 {
		entries = append(entries, stopEntry{id: id, stop: shell.Stop})
	}
	for id, client := range snapshot.qqOfficial {
		entries = append(entries, stopEntry{id: id, stop: client.Stop})
	}
	err := stopAdapters(ctx, entries)
	s.PublishSnapshot()
	s.hub.Close()
	return err
}

type stopEntry struct {
	id   string
	stop func(context.Context) error
}

func stopAdapters(ctx context.Context, entries []stopEntry) error {
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	failures := make([]error, len(entries))
	var stopped sync.WaitGroup
	for index, entry := range entries {
		stopped.Add(1)
		go func() {
			defer stopped.Done()
			if err := entry.stop(ctx); err != nil {
				failures[index] = fmt.Errorf("stop adapter %s: %w", entry.id, err)
			}
		}()
	}
	stopped.Wait()
	return errors.Join(failures...)
}
