package app

type schedulerHTTPServiceImpl struct {
	system    systemHTTPService
	scheduler schedulerEngineHTTPService
}

func newSchedulerHTTPService(system systemHTTPService, scheduler schedulerEngineHTTPService) *schedulerHTTPServiceImpl {
	return &schedulerHTTPServiceImpl{system: system, scheduler: scheduler}
}
