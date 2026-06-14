package manager

import (
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
)

type Delivery struct {
	RequestID    string
	Action       *runtimeaction.Action
	Result       map[string]any
	ErrorCode    string
	ErrorMessage string
	ErrorDetails map[string]any
}
