package tasks

func (e *Executor) run() {
	defer e.wg.Done()
	for {
		select {
		case <-e.baseCtx.Done():
			return
		case job := <-e.jobs:
			e.execute(job)
		}
	}
}

func (e *Executor) execute(job executorJob) {
	defer e.dropCancel(job.taskID)

	snapshot, ok := e.registry.Get(job.taskID)
	if !ok {
		return
	}
	if snapshot.Status == StatusCancelled {
		e.recordTaskMetric(snapshot.TaskType, "cancelled", 0)
		return
	}

	startedAt := e.now().UTC()
	e.registry.Update(job.taskID, Update{
		Status:    statusPtr(StatusRunning),
		Progress:  intP(0),
		StartedAt: &startedAt,
	})

	reporter := ProgressReporter{registry: e.registry, taskID: job.taskID}
	result, err := job.execute(job.ctx, reporter)

	now := e.now().UTC()
	duration := now.Sub(startedAt)
	if err != nil {
		var taskErr *TaskError
		if ok := isTaskError(err, &taskErr); ok {
			e.registry.Update(job.taskID, Update{
				Status:     statusPtr(StatusFailed),
				Summary:    strPtr(taskErr.Message),
				FinishedAt: &now,
				Error: &ErrorSummary{
					Code:    taskErr.Code,
					Message: taskErr.Message,
					Details: taskErr.Details,
				},
			})
		} else {
			code := "platform.internal_error"
			if job.ctx.Err() != nil {
				code = "platform.task_timeout"
			}
			e.registry.Update(job.taskID, Update{
				Status:     statusPtr(StatusFailed),
				Summary:    strPtr(err.Error()),
				FinishedAt: &now,
				Error: &ErrorSummary{
					Code:    code,
					Message: err.Error(),
				},
			})
		}
		outcome := "failed"
		if job.ctx.Err() != nil {
			outcome = "cancelled"
		}
		e.recordTaskMetric(snapshot.TaskType, outcome, duration)
		return
	}

	if result == nil {
		result = &ResultSummary{Summary: "完成"}
	}
	e.registry.Update(job.taskID, Update{
		Status:     statusPtr(StatusSucceeded),
		Progress:   intP(100),
		Summary:    strPtr(result.Summary),
		FinishedAt: &now,
		Result:     result,
	})
	e.recordTaskMetric(snapshot.TaskType, "succeeded", duration)
}
