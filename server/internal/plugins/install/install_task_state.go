package install

import "github.com/RayleaBot/RayleaBot/server/internal/tasks"

func (s *InstallService) failTask(taskID, code, message, summary string) {
	now := s.deps.now().UTC()
	s.registry.Update(taskID, tasks.Update{
		Status:     taskStatusPtr(tasks.StatusFailed),
		Summary:    stringPtr(summary),
		FinishedAt: &now,
		Error: &tasks.ErrorSummary{
			Code:    code,
			Message: message,
		},
	})
}

func (s *InstallService) dropCancel(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cancels, taskID)
}
