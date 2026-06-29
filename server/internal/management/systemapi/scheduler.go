package systemapi

type schedulerHTTPServiceImpl struct {
	system    SchedulerMetadataService
	scheduler SchedulerEngineService
}

func newSchedulerHTTPService(system SchedulerMetadataService, scheduler SchedulerEngineService) *schedulerHTTPServiceImpl {
	return &schedulerHTTPServiceImpl{system: system, scheduler: scheduler}
}
