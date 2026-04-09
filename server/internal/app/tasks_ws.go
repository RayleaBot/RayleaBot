package app

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type taskFrame struct {
	Channel   string        `json:"channel"`
	Type      string        `json:"type"`
	Timestamp string        `json:"timestamp"`
	Data      taskFrameData `json:"data"`
}

type taskFrameData struct {
	TaskID   string               `json:"task_id"`
	TaskType string               `json:"task_type"`
	Status   tasks.Status         `json:"status"`
	Progress int                  `json:"progress,omitempty"`
	Summary  string               `json:"summary"`
	Result   *tasks.ResultSummary `json:"result,omitempty"`
	Error    *tasks.ErrorSummary  `json:"error,omitempty"`
}

func (h *tasksWSHandler) handleTasksWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		framesCtx := conn.CloseRead(context.Background())
		updates, unsubscribe := h.tasks.Subscribe(8)
		defer unsubscribe()

		for _, snapshot := range h.tasks.List() {
			if err := wsjson.Write(framesCtx, conn, newTaskFrame(snapshot)); err != nil {
				return
			}
		}

		for {
			select {
			case <-framesCtx.Done():
				return
			case snapshot, ok := <-updates:
				if !ok {
					return
				}
				if err := wsjson.Write(framesCtx, conn, newTaskFrame(snapshot)); err != nil {
					return
				}
			}
		}
	}
}

func newTaskFrame(snapshot tasks.Snapshot) taskFrame {
	return taskFrame{
		Channel:   "tasks",
		Type:      "tasks.updated",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: taskFrameData{
			TaskID:   snapshot.TaskID,
			TaskType: snapshot.TaskType,
			Status:   snapshot.Status,
			Progress: snapshot.Progress,
			Summary:  snapshot.Summary,
			Result:   snapshot.Result,
			Error:    snapshot.Error,
		},
	}
}
