package view

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type DeadLetterResponse struct {
	EnteredAt        string `json:"entered_at"`
	CrashCount       int    `json:"crash_count"`
	LastErrorCode    string `json:"last_error_code,omitempty"`
	LastErrorMessage string `json:"last_error_message,omitempty"`
}

func BuildDeadLetter(snapshot plugins.Snapshot) *DeadLetterResponse {
	if snapshot.RuntimeState != "dead_letter" || snapshot.DeadLetter == nil {
		return nil
	}
	return &DeadLetterResponse{
		EnteredAt:        snapshot.DeadLetter.EnteredAt.UTC().Format(time.RFC3339Nano),
		CrashCount:       snapshot.DeadLetter.CrashCount,
		LastErrorCode:    strings.TrimSpace(snapshot.DeadLetter.LastErrorCode),
		LastErrorMessage: strings.TrimSpace(snapshot.DeadLetter.LastErrorMessage),
	}
}
