package system

import "github.com/RayleaBot/RayleaBot/server/internal/tasks"

type TaskStatus struct {
	TaskID    string       `json:"task_id"`
	Status    tasks.Status `json:"status"`
	ErrorCode string       `json:"error_code,omitempty"`
}

func (s *Service) GetTaskStatus(taskID string) (TaskStatus, bool) {
	if s.taskExecutor == nil {
		return TaskStatus{}, false
	}
	snapshot, ok := s.taskExecutor.Get(taskID)
	if !ok {
		return TaskStatus{}, false
	}
	result := TaskStatus{TaskID: snapshot.TaskID, Status: snapshot.Status}
	if snapshot.Error != nil {
		result.ErrorCode = snapshot.Error.Code
	}
	return result, true
}
