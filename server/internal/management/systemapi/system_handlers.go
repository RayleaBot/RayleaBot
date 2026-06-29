package systemapi

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	systemmodel "github.com/RayleaBot/RayleaBot/server/internal/system/model"
)

type SystemHandlers struct {
	system    CoreService
	scheduler SchedulerService
}

type CoreService interface {
	CurrentReadiness() health.ReadinessReport
	DiagnosticsSnapshot(context.Context) systemmodel.DiagnosticsSnapshot
	BuildDiagnosticsArchive(context.Context) ([]byte, error)
	SubmitSystemBackupTask() (string, error)
	ValidateRecoveryConfirmRequest([]string, string) *systemmodel.Error
	SubmitRecoveryRecheckTask() (string, *systemmodel.Error)
	SubmitRecoveryConfirmTask([]string, string, string) (string, *systemmodel.Error)
	SubmitRuntimeBootstrapTask([]string) (string, error)
}

type SchedulerMetadataService interface {
	SchedulerPluginName(string) string
	SchedulerTimezone() string
}

type SystemService interface {
	CoreService
	SchedulerMetadataService
}

type SchedulerService interface {
	ListJobs() (schedulerJobListResponse, bool)
	TriggerJob(context.Context, string) (schedulerJobTriggerResponse, *SystemHTTPError)
}

type SchedulerEngineService interface {
	Jobs() []scheduler.Job
	Trigger(context.Context, string) (scheduler.Job, error)
}

func NewSystemHandlers(system CoreService, schedulerEngine ...SchedulerEngineService) *SystemHandlers {
	var schedulerValue SchedulerService
	if len(schedulerEngine) > 0 {
		metadata, _ := system.(SchedulerMetadataService)
		schedulerValue = newSchedulerHTTPService(metadata, schedulerEngine[0])
	}
	return &SystemHandlers{system: system, scheduler: schedulerValue}
}

func NewSchedulerHandlers(metadata SchedulerMetadataService, schedulerEngine SchedulerEngineService) *SystemHandlers {
	return &SystemHandlers{scheduler: newSchedulerHTTPService(metadata, schedulerEngine)}
}

func (h *SystemHandlers) CurrentReadiness() health.ReadinessReport {
	if h == nil || h.system == nil {
		return health.ReadinessReport{Status: "failed", Reason: "system service unavailable"}
	}
	return h.system.CurrentReadiness()
}
