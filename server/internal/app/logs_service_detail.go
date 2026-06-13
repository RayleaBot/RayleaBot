package app

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func (s *logService) getLogSummary(ctx context.Context, logID string) (logging.Summary, error) {
	trimmedLogID := strings.TrimSpace(logID)
	if s != nil && s.repository != nil {
		item, err := s.repository.GetSummary(ctx, trimmedLogID)
		if err == nil {
			return item, nil
		}
		if err != logging.ErrLogNotFound {
			return logging.Summary{}, err
		}
		if item, ok := s.findStreamLogSummary(trimmedLogID); ok {
			return item, nil
		}
		return logging.Summary{}, logging.ErrLogNotFound
	}

	if item, ok := s.findStreamLogSummary(trimmedLogID); ok {
		return item, nil
	}

	if s == nil || s.stream == nil {
		return logging.Summary{}, logging.ErrLogNotFound
	}

	return logging.Summary{}, logging.ErrLogNotFound
}

func (s *logService) findStreamLogSummary(logID string) (logging.Summary, bool) {
	if s == nil || s.stream == nil || logID == "" {
		return logging.Summary{}, false
	}

	for _, item := range s.stream.Snapshot() {
		if item.LogID == logID {
			return item, true
		}
	}

	return logging.Summary{}, false
}
