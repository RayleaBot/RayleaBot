package managementhttp

import "github.com/RayleaBot/RayleaBot/server/internal/tasks"

type TaskHandlers struct {
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

func NewTaskHandlers(taskRegistry taskRegistryHTTPService, taskExecutor taskCanceller, pluginInstaller taskCanceller) *TaskHandlers {
	return &TaskHandlers{
		tasks:           taskRegistry,
		taskExecutor:    taskExecutor,
		pluginInstaller: pluginInstaller,
	}
}

func (h *TaskHandlers) SetPluginInstaller(installer taskCanceller) {
	if h == nil {
		return
	}
	h.pluginInstaller = installer
}
