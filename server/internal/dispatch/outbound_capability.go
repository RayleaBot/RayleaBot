package dispatch

import "context"

func (d *Dispatcher) capabilityDeclared(ctx context.Context, pluginID string, capability string) bool {
	if d == nil {
		return false
	}
	d.mu.RLock()
	checker := d.capabilityChecker
	d.mu.RUnlock()
	if checker == nil {
		return true
	}
	return checker(ctx, pluginID, capability)
}
