package app

const (
	codePermissionDenied   = "permission.denied"
	codeInvalidRequest     = "platform.invalid_request"
	codeResourceMissing    = "platform.resource_missing"
	codeInternalError      = "platform.internal_error"
	codeTaskNotCancellable = "platform.task_not_cancellable"
)

type taskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}
