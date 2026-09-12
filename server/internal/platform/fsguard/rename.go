package fsguard

import (
	"context"
	"errors"
	"os"
	"time"
)

// RenameRetryOptions controls bounded retries and the caller's filesystem hooks.
// Nil hooks use os.Rename, Windows sharing-error detection and a cancellable delay.
type RenameRetryOptions struct {
	Attempts  int
	Delay     time.Duration
	Rename    func(string, string) error
	Retryable func(error) bool
	Wait      func(context.Context) error
}

// RenameWithRetry retries temporary Windows rename failures. Attempts includes
// the initial operation; cancellation preserves the most recent filesystem error.
func RenameWithRetry(ctx context.Context, source, target string, options RenameRetryOptions) error {
	if options.Attempts < 1 {
		return errors.New("rename attempts must be positive")
	}
	rename := options.Rename
	if rename == nil {
		rename = os.Rename
	}
	retryable := options.Retryable
	if retryable == nil {
		retryable = isRetryableRenameError
	}
	wait := options.Wait
	if wait == nil {
		wait = func(ctx context.Context) error {
			timer := time.NewTimer(options.Delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}
	var lastErr error
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return errors.Join(lastErr, err)
		}
		lastErr = rename(source, target)
		if lastErr == nil {
			return nil
		}
		if attempt == options.Attempts || !retryable(lastErr) {
			return lastErr
		}
		if err := wait(ctx); err != nil {
			return errors.Join(lastErr, err)
		}
	}
}
