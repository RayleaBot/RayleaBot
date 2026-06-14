package apphost

import (
	"context"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type schedulerTriggerProxy struct {
	mu      sync.RWMutex
	handler func(context.Context, scheduler.Job)
}

func newSchedulerTriggerProxy() *schedulerTriggerProxy {
	return &schedulerTriggerProxy{}
}

func (p *schedulerTriggerProxy) Set(handler func(context.Context, scheduler.Job)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handler = handler
}

func (p *schedulerTriggerProxy) Handle(ctx context.Context, job scheduler.Job) {
	if p == nil {
		return
	}
	p.mu.RLock()
	handler := p.handler
	p.mu.RUnlock()
	if handler != nil {
		handler(ctx, job)
	}
}
