package plugins

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (s *UninstallService) Accept(_ context.Context, pluginID string) (string, error) {
	summary := fmt.Sprintf("uninstall plugin: %s", pluginID)
	taskID, err := s.registry.Create("plugin.uninstall", summary)
	if err != nil {
		return "", err
	}

	runCtx, cancel := context.WithTimeout(s.baseCtx, 5*time.Minute)
	s.mu.Lock()
	s.cancels[taskID] = cancel
	s.mu.Unlock()

	select {
	case s.jobs <- uninstallJob{taskID: taskID, pluginID: pluginID, ctx: runCtx}:
		return taskID, nil
	case <-s.baseCtx.Done():
		cancel()
		return "", errors.New("uninstall service is shutting down")
	}
}

func (s *UninstallService) Close() error {
	if s == nil {
		return nil
	}
	s.baseCancel()

	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.cancels))
	for _, cancel := range s.cancels {
		cancels = append(cancels, cancel)
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}

	s.wg.Wait()
	return nil
}

func (s *UninstallService) run() {
	defer s.wg.Done()
	for {
		select {
		case <-s.baseCtx.Done():
			return
		case job := <-s.jobs:
			s.execute(job)
		}
	}
}
