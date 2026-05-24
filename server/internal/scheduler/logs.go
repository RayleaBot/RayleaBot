package scheduler

import (
	"fmt"
	"strings"
	"time"
)

func DisplayLabel(values ...string) string {
	for _, value := range values {
		if label := strings.TrimSpace(value); label != "" {
			return label
		}
	}
	return ""
}

func DisplayMessage(pluginName, taskName, logLabel, status string) string {
	parts := []string{
		strings.TrimSpace(pluginName),
		strings.TrimSpace(taskName),
	}
	if label := strings.TrimSpace(logLabel); label != "" {
		parts = append(parts, label)
	}
	parts = append(parts, strings.TrimSpace(status))
	return "【" + strings.Join(parts, "｜") + "】"
}

func FormatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	return fmt.Sprintf("%dms", duration.Milliseconds())
}
