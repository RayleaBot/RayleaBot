package scheduler

import "context"

type Repository interface {
	SaveJob(ctx context.Context, job Job) error
	LoadJobs(ctx context.Context) ([]Job, error)
	DeleteJob(ctx context.Context, jobID string) error
	DeleteJobsByPlugin(ctx context.Context, pluginID string) error
	RecordJobRunResult(ctx context.Context, result RunResult) error
	UpdateJobSchedule(ctx context.Context, job Job) error
}
