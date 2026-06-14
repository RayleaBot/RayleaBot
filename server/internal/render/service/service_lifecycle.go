package service

import (
	"strings"

	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
)

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	releaseWorkers := s.acquireAllWorkerSlots()
	defer releaseWorkers()

	s.mu.Lock()
	runner := s.runner
	s.runner = nil
	s.mu.Unlock()
	return closeRenderRunner(runner)
}

func (s *Service) RefreshBrowserPath(browserPath string) {
	if s == nil {
		return
	}

	trimmed := strings.TrimSpace(browserPath)
	s.mu.Lock()
	s.browserPath = trimmed
	oldRunner := s.runner
	browserArgs := append([]string(nil), s.browserArgs...)
	replaceDefaultRunner := renderbrowser.IsChromiumRunner(oldRunner)
	s.mu.Unlock()

	if !replaceDefaultRunner {
		return
	}

	releaseWorkers := s.acquireAllWorkerSlots()
	defer releaseWorkers()

	s.mu.Lock()
	if s.runner != oldRunner {
		s.mu.Unlock()
		return
	}
	s.runner = renderbrowser.NewChromiumRunner(renderbrowser.ChromiumOptions{
		BrowserPath: trimmed,
		BrowserArgs: browserArgs,
	})
	s.mu.Unlock()
	_ = closeRenderRunner(oldRunner)
}

func (s *Service) currentRunner() renderbrowser.Runner {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runner
}

func (s *Service) acquireAllWorkerSlots() func() {
	if s == nil || s.workerSem == nil {
		return func() {}
	}
	count := cap(s.workerSem)
	for i := 0; i < count; i++ {
		s.workerSem <- struct{}{}
	}
	return func() {
		for i := 0; i < count; i++ {
			<-s.workerSem
		}
	}
}

func closeRenderRunner(runner renderbrowser.Runner) error {
	closeable, ok := runner.(closeableRunner)
	if !ok || closeable == nil {
		return nil
	}
	return closeable.Close()
}
