package app

import "github.com/RayleaBot/RayleaBot/server/internal/tasks"

type taskHTTPHandlers struct {
	tasks           taskRegistryHTTPService
	taskExecutor    taskCanceller
	pluginInstaller taskCanceller
}

type taskRegistryHTTPService interface {
	List() []tasks.Snapshot
	Get(string) (tasks.Snapshot, bool)
	Update(string, tasks.Update) (tasks.Snapshot, bool)
}

type taskCanceller interface {
	Cancel(string) bool
}

func newTaskHTTPHandlers(taskRegistry taskRegistryHTTPService, taskExecutor taskCanceller, pluginInstaller taskCanceller) *taskHTTPHandlers {
	return &taskHTTPHandlers{
		tasks:           taskRegistry,
		taskExecutor:    taskExecutor,
		pluginInstaller: pluginInstaller,
	}
}
