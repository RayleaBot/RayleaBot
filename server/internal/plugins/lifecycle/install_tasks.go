package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type installTaskError struct {
	Code    string
	Message string
	Summary string
}

func (e *installTaskError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func installError(code, message, summary string) error {
	return &installTaskError{
		Code:    code,
		Message: message,
		Summary: summary,
	}
}

func InstallErrorCode(err error) string {
	var installErr *installTaskError
	if errors.As(err, &installErr) {
		return installErr.Code
	}
	return ""
}

func (s *InstallService) failTask(taskID, code, message, summary string, details ...map[string]any) {
	now := s.deps.now().UTC()
	var errorDetails map[string]any
	if len(details) > 0 {
		errorDetails = details[0]
	}
	s.registry.Update(taskID, tasks.Update{
		Status:     taskStatusPtr(tasks.StatusFailed),
		Summary:    stringPtr(summary),
		FinishedAt: &now,
		Error: &tasks.ErrorSummary{
			Code:    code,
			Message: message,
			Details: errorDetails,
		},
	})
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func taskStatusPtr(status tasks.Status) *tasks.Status {
	return &status
}

func (s *InstallService) cleanupInstallInspection(job installJob, cause error) error {
	if job.inspection == nil {
		return cause
	}
	var failure *operationError
	errors.As(cause, &failure)
	// A failed rollback may retain the only good package in workingRoot/previous.
	if failure != nil && failure.state == "rollback_failed" {
		return cause
	}
	cleanupErr := s.deps.removeAll(job.inspection.workingRoot)
	if cleanupErr == nil {
		return cause
	}
	if failure == nil {
		state := "unchanged"
		if cause == nil {
			state = "committed"
		}
		failure = &operationError{state: state}
		failure.add("install", cause)
	}
	failure.add("cleanup", cleanupErr)
	return failure
}

func (s *InstallService) reportInstallResult(job installJob, pluginName string, err error) {
	var operationErr *operationError
	errors.As(err, &operationErr)
	if operationErr != nil && operationErr.taskMustFail(err) {
		s.failTask(job.taskID, codePluginInstallFailed, operationErr.message(), "插件“"+pluginName+"”："+operationErr.message(), operationErr.details())
		return
	}
	switch {
	case err == nil:
		now := s.deps.now().UTC()
		s.registry.Update(job.taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusSucceeded),
			Progress:   intPtr(100),
			Summary:    stringPtr("插件“" + pluginName + "”安装完成"),
			FinishedAt: &now,
			Result: &tasks.ResultSummary{
				Summary: "插件“" + pluginName + "”安装完成",
				Details: map[string]any{"plugin_id": job.inspection.snapshot.PluginID, "plugin_name": pluginName, "source_type": job.request.SourceType, "source_ref": job.request.Source},
			},
		})
	case errors.Is(err, context.Canceled):
		now := s.deps.now().UTC()
		s.registry.Update(job.taskID, tasks.Update{
			Status:     taskStatusPtr(tasks.StatusCancelled),
			Summary:    stringPtr("插件“" + pluginName + "”安装已取消"),
			FinishedAt: &now,
		})
	case errors.Is(err, context.DeadlineExceeded):
		s.failTask(job.taskID, codePlatformTaskTimeout, "插件安装超时", "插件“"+pluginName+"”安装超时")
	default:
		var installErr *installTaskError
		if errors.As(err, &installErr) {
			s.failTask(job.taskID, installErr.Code, installErr.Message, "插件“"+pluginName+"”："+installErr.Summary)
			return
		}
		s.failTask(job.taskID, codePluginInstallFailed, "插件安装失败", "插件“"+pluginName+"”安装失败")
	}
}

func installPluginName(snapshot plugins.Snapshot) string {
	if name := strings.TrimSpace(snapshot.Name); name != "" {
		return name
	}
	return snapshot.PluginID
}
