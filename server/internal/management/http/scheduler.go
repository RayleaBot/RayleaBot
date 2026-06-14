package managementhttp

type schedulerHTTPServiceImpl struct {
	system    SystemService
	scheduler SchedulerEngineService
}

func newSchedulerHTTPService(system SystemService, scheduler SchedulerEngineService) *schedulerHTTPServiceImpl {
	return &schedulerHTTPServiceImpl{system: system, scheduler: scheduler}
}
