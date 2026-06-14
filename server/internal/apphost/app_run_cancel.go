package apphost

import "context"

func (a *App) setRunCancel(cancel context.CancelFunc) {
	a.process.runCancelMu.Lock()
	defer a.process.runCancelMu.Unlock()
	a.process.runCancel = cancel
}

func (a *App) clearRunCancel() {
	a.process.runCancelMu.Lock()
	defer a.process.runCancelMu.Unlock()
	a.process.runCancel = nil
}

func (a *App) requestShutdown() {
	if a == nil {
		return
	}

	a.process.shuttingDown.Store(true)
	a.process.shutdownOnce.Do(func() {
		a.process.runCancelMu.Lock()
		cancel := a.process.runCancel
		a.process.runCancelMu.Unlock()
		if cancel != nil {
			cancel()
		}
	})
}
