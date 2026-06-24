package app

import (
	"context"
	"sync"
)

type runSupervisor struct {
	ctx    context.Context
	cancel context.CancelFunc
	errCh  chan error
	once   sync.Once
}

func newRunSupervisor(parent context.Context) *runSupervisor {
	ctx, cancel := context.WithCancel(parent)
	return &runSupervisor{
		ctx:    ctx,
		cancel: cancel,
		errCh:  make(chan error, 1),
	}
}

func (s *runSupervisor) Context() context.Context {
	if s == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *runSupervisor) Cancel() {
	if s == nil {
		return
	}
	s.cancel()
}

func (s *runSupervisor) Go(run func(context.Context) error) {
	if s == nil || run == nil {
		return
	}
	go func() {
		if err := run(s.ctx); err != nil {
			s.report(err)
		}
	}()
}

func (s *runSupervisor) GoCritical(run func(context.Context) error) {
	if s == nil || run == nil {
		return
	}
	go func() {
		s.once.Do(func() {
			s.errCh <- run(s.ctx)
		})
	}()
}

func (s *runSupervisor) report(err error) {
	if s == nil || err == nil {
		return
	}
	s.once.Do(func() {
		s.errCh <- err
	})
}

func (s *runSupervisor) Errors() <-chan error {
	if s == nil {
		ch := make(chan error)
		close(ch)
		return ch
	}
	return s.errCh
}
