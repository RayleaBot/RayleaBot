package desktop

import (
	"context"
	"time"
)

func (c *Coordinator) monitor(ctx context.Context) {
	statusTicker := time.NewTicker(2 * time.Second)
	releaseTicker := time.NewTicker(releaseCacheTTL)
	defer statusTicker.Stop()
	defer releaseTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-statusTicker.C:
			if !c.operationMu.TryLock() {
				continue
			}
			_ = c.refreshCurrentPassive()
			c.operationMu.Unlock()
		case <-releaseTicker.C:
			go c.refreshRelease(false)
		}
	}
}
