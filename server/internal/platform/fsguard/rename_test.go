package fsguard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenameRetryOutcomes(t *testing.T) {
	t.Parallel()
	transient := errors.New("temporary file use")
	permanent := errors.New("invalid rename")
	for _, tt := range []struct {
		name     string
		failures int
		failure  error
		wantErr  error
		calls    int
	}{
		{"released before limit", 2, transient, nil, 3},
		{"still held at limit", 3, transient, transient, 3},
		{"permanent error", 3, permanent, permanent, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			source, target := filepath.Join(root, "candidate"), filepath.Join(root, "installed")
			if err := os.WriteFile(source, []byte("verified package"), 0o600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			err := RenameWithRetry(t.Context(), source, target, RenameRetryOptions{
				Attempts: 3,
				Rename: func(from, to string) error {
					calls++
					if calls <= tt.failures {
						return tt.failure
					}
					return os.Rename(from, to)
				},
				Retryable: func(err error) bool { return errors.Is(err, transient) },
				Wait:      func(context.Context) error { return nil },
			})
			if !errors.Is(err, tt.wantErr) || calls != tt.calls {
				t.Fatalf("rename returned %v after %d calls", err, calls)
			}
			present, absent := target, source
			if tt.wantErr != nil {
				present, absent = source, target
			}
			if content, err := os.ReadFile(present); err != nil || string(content) != "verified package" {
				t.Fatalf("package contents: %q, %v", content, err)
			}
			if _, err := os.Stat(absent); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected path %s: %v", absent, err)
			}
		})
	}
}

func TestRenameRetryCancellationPreservesCause(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	locked := errors.New("file is held")
	err := RenameWithRetry(ctx, "source", "target", RenameRetryOptions{
		Attempts: 10, Delay: time.Hour,
		Rename: func(string, string) error {
			cancel()
			return locked
		},
		Retryable: func(error) bool { return true },
	})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, locked) {
		t.Fatalf("cancellation lost filesystem cause: %v", err)
	}
}

func TestRenameRetryDoesNotRunWithoutAnAttempt(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, attempts := range []int{0, 1} {
		err := RenameWithRetry(ctx, "source", "target", RenameRetryOptions{
			Attempts: attempts,
			Rename: func(string, string) error {
				t.Fatal("rename ran without an available attempt")
				return nil
			},
		})
		if err == nil || (attempts > 0 && !errors.Is(err, context.Canceled)) {
			t.Fatalf("attempts=%d: %v", attempts, err)
		}
	}
}
