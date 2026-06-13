package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type systemHTTPHandlers struct {
	system    systemHTTPService
	scheduler schedulerHTTPService
}

type systemHTTPService interface {
	CurrentReadiness() health.ReadinessReport
	buildDiagnosticsArchive(context.Context) ([]byte, error)
	submitSystemBackupTask() (string, error)
	validateRecoveryConfirmRequest([]string, string) *systemHTTPError
	submitRecoveryRecheckTask() (string, *systemHTTPError)
	submitRecoveryConfirmTask([]string, string, string) (string, *systemHTTPError)
	submitRuntimeBootstrapTask([]string) (string, error)
	schedulerPluginName(string) string
	schedulerTimezone() string
}

type schedulerHTTPService interface {
	ListJobs() (schedulerJobListResponse, bool)
	TriggerJob(context.Context, string) (schedulerJobTriggerResponse, *systemHTTPError)
}

type schedulerEngineHTTPService interface {
	Jobs() []scheduler.Job
	Trigger(context.Context, string) (scheduler.Job, error)
}

func newSystemHTTPHandlers(system systemHTTPService, schedulerEngine ...schedulerEngineHTTPService) *systemHTTPHandlers {
	var schedulerValue schedulerHTTPService
	if len(schedulerEngine) > 0 {
		schedulerValue = newSchedulerHTTPService(system, schedulerEngine[0])
	}
	return &systemHTTPHandlers{system: system, scheduler: schedulerValue}
}
