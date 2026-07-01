package uninstall

import "github.com/RayleaBot/RayleaBot/server/internal/tasks"

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func taskStatusPtr(status tasks.Status) *tasks.Status {
	return &status
}
