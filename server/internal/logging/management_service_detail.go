package logging

import (
	"context"
	"strings"
)

func (s *ManagementService) GetLogSummary(ctx context.Context, logID string) (Summary, error) {
	trimmedLogID := strings.TrimSpace(logID)
	if s != nil && s.repository != nil {
		item, err := s.repository.GetSummary(ctx, trimmedLogID)
		if err == nil {
			return item, nil
		}
		if err != ErrLogNotFound {
			return Summary{}, err
		}
		if item, ok := s.findStreamLogSummary(trimmedLogID); ok {
			return item, nil
		}
		return Summary{}, ErrLogNotFound
	}

	if item, ok := s.findStreamLogSummary(trimmedLogID); ok {
		return item, nil
	}

	if s == nil || s.stream == nil {
		return Summary{}, ErrLogNotFound
	}

	return Summary{}, ErrLogNotFound
}

func (s *ManagementService) findStreamLogSummary(logID string) (Summary, bool) {
	if s == nil || s.stream == nil || logID == "" {
		return Summary{}, false
	}

	for _, item := range s.stream.Snapshot() {
		if item.LogID == logID {
			return item, true
		}
	}

	return Summary{}, false
}
