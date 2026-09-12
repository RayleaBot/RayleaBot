package app

import (
	"time"

	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type schedulerDiagnostics struct {
	scheduler *scheduler.Engine
}

func (d schedulerDiagnostics) Timezone() string {
	return d.scheduler.Timezone()
}

func (d schedulerDiagnostics) DiagnosticsScheduler() systemsvc.DiagnosticsScheduler {
	result := systemsvc.DiagnosticsScheduler{}
	if d.scheduler == nil {
		return result
	}
	now := time.Now().UTC()
	result.Running = d.scheduler.RunningCount()
	for _, job := range d.scheduler.Jobs() {
		result.Total++
		if job.Enabled {
			result.Enabled++
			if !job.NextRun.After(now) {
				result.Pending++
			}
		} else {
			result.Disabled++
		}
		if job.LastError != nil {
			result.Failed++
		}
	}
	return result
}
