package management

import (
	"context"

	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
)

type diagnosticsTestSystem struct {
	snapshot systemsvc.DiagnosticsSnapshot
}

func (s diagnosticsTestSystem) GetTaskStatus(string) (systemsvc.TaskStatus, bool) {
	return systemsvc.TaskStatus{}, false
}

func (s diagnosticsTestSystem) CurrentReadiness() systemsvc.ReadinessReport {
	return systemsvc.ReadinessReport{Status: "ready"}
}

func (s diagnosticsTestSystem) DiagnosticsSnapshot(context.Context) systemsvc.DiagnosticsSnapshot {
	return s.snapshot
}

func (s diagnosticsTestSystem) BuildDiagnosticsArchive(context.Context) ([]byte, error) {
	return nil, nil
}

func (s diagnosticsTestSystem) SubmitSystemBackupTask() (string, error) {
	return "", nil
}

func (s diagnosticsTestSystem) SubmitRuntimeBootstrapTask([]string) (string, error) {
	return "", nil
}
