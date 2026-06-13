package plugininstall

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *InstallService) Accept(_ context.Context, request InstallRequest) (string, error) {
	taskID, err := s.registry.Create("plugin.install", "install plugin from "+request.SourceType+": "+request.Source)
	if err != nil {
		return "", err
	}

	runCtx, cancel := context.WithTimeout(s.baseCtx, s.timeout)
	s.mu.Lock()
	s.cancels[taskID] = cancel
	s.mu.Unlock()

	select {
	case s.jobs <- installJob{taskID: taskID, request: request, ctx: runCtx}:
		return taskID, nil
	case <-s.baseCtx.Done():
		cancel()
		s.registry.Update(taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusFailed),
			FinishedAt: timePtr(s.deps.now().UTC()),
			Summary:    stringPtr("后台安装执行器不可用"),
			Error: &tasks.ErrorSummary{
				Code:    "platform.internal_error",
				Message: "安装执行器不可用",
			},
		})
		return "", errors.New("install service is shutting down")
	}
}

func (s *InstallService) Cancel(taskID string) bool {
	snapshot, ok := s.registry.Get(taskID)
	if !ok || snapshot.TaskType != "plugin.install" {
		return false
	}
	if snapshot.Status != tasks.StatusPending && snapshot.Status != tasks.StatusRunning {
		return false
	}

	s.mu.Lock()
	cancel, ok := s.cancels[taskID]
	s.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}

	cancel()
	if snapshot.Status == tasks.StatusPending {
		now := s.deps.now().UTC()
		s.registry.Update(taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusCancelled),
			Summary:    stringPtr("插件安装已取消"),
			FinishedAt: &now,
		})
		s.dropCancel(taskID)
	}

	return true
}

func (s *InstallService) SetAfterSuccess(fn func(string) error) {
	if s == nil {
		return
	}
	s.afterSuccess = fn
}

func (s *InstallService) SetRenderTemplateValidator(fn func(plugins.Snapshot) error) {
	if s == nil {
		return
	}
	s.validateRenderTemplates = fn
}

func (s *InstallService) Close() error {
	if s == nil {
		return nil
	}

	s.baseCancel()

	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.cancels))
	for _, cancel := range s.cancels {
		cancels = append(cancels, cancel)
	}
	s.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}

	s.wg.Wait()
	return nil
}

func (s *InstallService) run() {
	defer s.wg.Done()

	for {
		select {
		case <-s.baseCtx.Done():
			return
		case job := <-s.jobs:
			s.execute(job)
		}
	}
}

func (s *InstallService) execute(job installJob) {
	defer s.dropCancel(job.taskID)

	snapshot, ok := s.registry.Get(job.taskID)
	if !ok {
		return
	}
	if snapshot.Status == tasks.StatusCancelled {
		return
	}

	startedAt := s.deps.now().UTC()
	s.registry.Update(job.taskID, tasks.Update{
		Status:    taskStatusPtr(tasks.StatusRunning),
		Progress:  intPtr(5),
		Summary:   stringPtr("准备安装源"),
		StartedAt: &startedAt,
	})

	err := s.runInstall(job)
	switch {
	case err == nil:
		now := s.deps.now().UTC()
		s.registry.Update(job.taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusSucceeded),
			Progress:   intPtr(100),
			Summary:    stringPtr("插件安装完成"),
			FinishedAt: &now,
			Result: &tasks.ResultSummary{
				Summary: "插件已安装并刷新插件目录索引",
			},
		})
	case errors.Is(err, context.Canceled):
		now := s.deps.now().UTC()
		s.registry.Update(job.taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusCancelled),
			Summary:    stringPtr("插件安装已取消"),
			FinishedAt: &now,
		})
	case errors.Is(err, context.DeadlineExceeded):
		s.failTask(job.taskID, codePlatformTaskTimeout, "插件安装超时", "插件安装超时")
	default:
		var installErr *installTaskError
		if errors.As(err, &installErr) {
			s.failTask(job.taskID, installErr.Code, installErr.Message, installErr.Summary)
			return
		}
		s.failTask(job.taskID, codePluginInstallFailed, "插件安装失败", "插件安装失败")
	}
}
