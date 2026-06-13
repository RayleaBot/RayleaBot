package logging

import (
	"context"
	"fmt"
	"io"
	"time"
)

func (s *Stream) SetRepository(repository Repository, retentionDays int) {
	s.mu.Lock()
	s.repository = repository
	s.retentionDays = retentionDays
	spool := s.spool
	s.mu.Unlock()

	if repository != nil && spool != nil && spool.HasEntries() {
		s.signalFlush()
	}
}

func (s *Stream) ConfigureSpool(queue *SpoolQueue, stderr io.Writer) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.spool = queue
	if stderr != nil {
		s.stderr = stderr
	}
	startLoop := queue != nil && !s.flushLoopStarted && !s.flushLoopClosed
	if startLoop {
		s.flushLoopStarted = true
		s.flushWG.Add(1)
	}
	s.mu.Unlock()

	if startLoop {
		go s.flushLoop()
	}
	if queue != nil && queue.HasEntries() {
		s.signalFlush()
	}
}

func (s *Stream) FlushSpool(ctx context.Context) error {
	return s.flushSpool(ctx, true)
}

func (s *Stream) Close() {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.flushLoopClosed {
		s.mu.Unlock()
		return
	}
	waitForFlushLoop := s.flushLoopStarted
	s.flushLoopClosed = true
	close(s.flushStop)
	s.mu.Unlock()

	if waitForFlushLoop {
		s.flushWG.Wait()
	}
}

func (s *Stream) flushLoop() {
	defer s.flushWG.Done()

	ticker := time.NewTicker(s.flushTicker)
	defer ticker.Stop()

	for {
		select {
		case <-s.flushStop:
			return
		case <-ticker.C:
		case <-s.flushNotify:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.flushSpool(ctx, false); err != nil {
			s.reportPersistenceFailure("management log spool flush failed: %v", err)
		}
		cancel()
	}
}

func (s *Stream) flushSpool(ctx context.Context, reportError bool) error {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	repository := s.repository
	retentionDays := s.retentionDays
	spool := s.spool
	s.mu.RUnlock()

	if repository == nil || spool == nil || !spool.HasEntries() {
		return nil
	}

	result, err := spool.Flush(ctx, repository)
	if err != nil {
		if reportError {
			s.reportPersistenceFailure("management log spool flush failed: %v", err)
		}
		return err
	}
	if result.Flushed > 0 && retentionDays > 0 {
		cutoff := s.now().AddDate(0, 0, -retentionDays)
		if pruneErr := repository.PruneOlderThan(ctx, cutoff); pruneErr != nil {
			if reportError {
				s.reportPersistenceFailure("management log prune after spool flush failed: %v", pruneErr)
			}
			return pruneErr
		}
	}
	return nil
}

func (s *Stream) signalFlush() {
	if s == nil {
		return
	}
	select {
	case s.flushNotify <- struct{}{}:
	default:
	}
}

func (s *Stream) reportPersistenceFailure(format string, args ...any) {
	if s == nil {
		return
	}

	s.mu.RLock()
	stderr := s.stderr
	s.mu.RUnlock()
	if stderr == nil {
		return
	}

	now := s.now()
	s.diagnosticMu.Lock()
	if !s.lastDiagnostic.IsZero() && now.Sub(s.lastDiagnostic) < 10*time.Second {
		s.diagnosticMu.Unlock()
		return
	}
	s.lastDiagnostic = now
	s.diagnosticMu.Unlock()

	_, _ = fmt.Fprintf(stderr, "rayleabot logging persistence: "+format+"\n", args...)
}
