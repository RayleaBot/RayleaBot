package tasks

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	if snapshot.StartedAt != nil {
		startedAt := *snapshot.StartedAt
		cloned.StartedAt = &startedAt
	}
	if snapshot.FinishedAt != nil {
		finishedAt := *snapshot.FinishedAt
		cloned.FinishedAt = &finishedAt
	}
	cloned.Result = cloneResult(snapshot.Result)
	cloned.Error = cloneError(snapshot.Error)
	return cloned
}

func cloneResult(result *ResultSummary) *ResultSummary {
	if result == nil {
		return nil
	}

	cloned := &ResultSummary{
		Summary: result.Summary,
	}
	if result.Details != nil {
		cloned.Details = cloneMap(result.Details)
	}
	return cloned
}

func cloneError(errSummary *ErrorSummary) *ErrorSummary {
	if errSummary == nil {
		return nil
	}

	cloned := &ErrorSummary{
		Code:    errSummary.Code,
		Message: errSummary.Message,
	}
	if errSummary.Details != nil {
		cloned.Details = cloneMap(errSummary.Details)
	}
	return cloned
}

func cloneMap(source map[string]any) map[string]any {
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
