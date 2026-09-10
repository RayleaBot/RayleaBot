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
	for _, shell := range s.oneBotShells {
		shell.Start(ctx)
	}
	for _, client := range s.qqClients {
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
	type stopEntry struct {
		id   string
		stop func(context.Context) error
	}
	entries := make([]stopEntry, 0, len(s.oneBotShells)+len(s.qqClients))
	for id, shell := range s.oneBotShells {
		entries = append(entries, stopEntry{id: id, stop: shell.Stop})
	}
	for id, client := range s.qqClients {
		entries = append(entries, stopEntry{id: id, stop: client.Stop})
	}
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
	s.PublishSnapshot()
	s.hub.Close()
	return errors.Join(failures...)
}
