package service

import (
	"strings"

	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
)

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	return s.worker.Close()
}

func (s *Service) RefreshBrowserPath(browserPath string) {
	if s == nil {
		return
	}

	trimmed := strings.TrimSpace(browserPath)
	s.mu.Lock()
	s.browserPath = trimmed
	browserArgs := append([]string(nil), s.browserArgs...)
	s.mu.Unlock()

	s.worker.RefreshChromiumRunner(trimmed, browserArgs)
}

func (s *Service) currentRunner() renderbrowser.Runner {
	if s == nil {
		return nil
	}
	return s.worker.CurrentRunner()
}
