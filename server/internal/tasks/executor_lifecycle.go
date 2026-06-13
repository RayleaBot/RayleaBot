package tasks

import (
	"context"
)

func (e *Executor) Submit(taskType, summary string, fn ExecuteFunc) (string, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return "", context.Canceled
	}
	e.mu.Unlock()

	taskID, err := e.registry.Create(taskType, summary)
	if err != nil {
		return "", err
	}

	runCtx, cancel := context.WithTimeout(e.baseCtx, e.timeout)
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		cancel()
		return "", context.Canceled
	}
	e.cancels[taskID] = cancel
	e.mu.Unlock()

	select {
	case e.jobs <- executorJob{taskID: taskID, execute: fn, ctx: runCtx}:
		return taskID, nil
	case <-e.baseCtx.Done():
		cancel()
		return "", context.Canceled
	}
}

func (e *Executor) Cancel(taskID string) bool {
	snapshot, ok := e.registry.Get(taskID)
	if !ok {
		return false
	}
	if snapshot.Status != StatusPending && snapshot.Status != StatusRunning {
		return false
	}
	e.mu.Lock()
	cancel, ok := e.cancels[taskID]
	e.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	if snapshot.Status == StatusPending {
		now := e.now().UTC()
		e.registry.Update(taskID, Update{
			Status:     statusPtr(StatusCancelled),
			Summary:    strPtr("任务已取消"),
			FinishedAt: &now,
		})
		e.dropCancel(taskID)
	}
	return true
}

func (e *Executor) Close() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		e.wg.Wait()
		return nil
	}
	e.closed = true
	for _, cancel := range e.cancels {
		cancel()
	}
	e.mu.Unlock()
	e.baseCancel()
	e.wg.Wait()
	return nil
}

func (e *Executor) dropCancel(taskID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.cancels, taskID)
}
