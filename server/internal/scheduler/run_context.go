package scheduler

import (
	"context"
	"time"
)

type RunContext struct {
	JobID      string
	Revision   uint64
	PluginName string
	TaskName   string
	LogLabel   string
	StartedAt  time.Time
	Recorder   RunRecorder
}

// RunRecorder persists a scheduled invocation's outcome.
type RunRecorder interface {
	RecordRunResult(context.Context, RunResult) error
}
