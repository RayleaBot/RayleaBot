package managementhttp

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type SystemHandlers struct {
	system    SystemService
	scheduler SchedulerService
}

type SystemService interface {
	CurrentReadiness() health.ReadinessReport
	BuildDiagnosticsArchive(context.Context) ([]byte, error)
	SubmitSystemBackupTask() (string, error)
	ValidateRecoveryConfirmRequest([]string, string) *SystemHTTPError
	SubmitRecoveryRecheckTask() (string, *SystemHTTPError)
	SubmitRecoveryConfirmTask([]string, string, string) (string, *SystemHTTPError)
	SubmitRuntimeBootstrapTask([]string) (string, error)
	SchedulerPluginName(string) string
	SchedulerTimezone() string
}

type SchedulerService interface {
	ListJobs() (schedulerJobListResponse, bool)
	TriggerJob(context.Context, string) (schedulerJobTriggerResponse, *SystemHTTPError)
}

type SchedulerEngineService interface {
	Jobs() []scheduler.Job
	Trigger(context.Context, string) (scheduler.Job, error)
}

func NewSystemHandlers(system SystemService, schedulerEngine ...SchedulerEngineService) *SystemHandlers {
	var schedulerValue SchedulerService
	if len(schedulerEngine) > 0 {
		schedulerValue = newSchedulerHTTPService(system, schedulerEngine[0])
	}
	return &SystemHandlers{system: system, scheduler: schedulerValue}
}

func (h *SystemHandlers) CurrentReadiness() health.ReadinessReport {
	if h == nil || h.system == nil {
		return health.ReadinessReport{Status: "failed", Reason: "system service unavailable"}
	}
	return h.system.CurrentReadiness()
}
