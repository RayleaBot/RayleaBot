package render

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerQueueCancellationKeepsIdentity(t *testing.T) {
	queued := make(chan struct{}, 1)
	w := NewWorker(WorkerConfig{WorkerCount: 1, QueueMaxLength: 1, QueueWaitTimeout: time.Minute, OnQueueDepth: func(depth int) {
		if depth == 2 {
			queued <- struct{}{}
		}
	}})
	defer func() { _ = w.Close() }()
	release, err := w.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := w.Acquire(ctx); done <- err }()
	<-queued
	cancel()
	err = <-done
	var renderErr *Error
	if !errors.Is(err, context.Canceled) || errors.As(err, &renderErr) {
		t.Fatalf("queued cancellation = %v", err)
	}
	release()
	if acquired, err := w.Acquire(ctx); acquired != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled acquisition = %v", err)
	}
}
