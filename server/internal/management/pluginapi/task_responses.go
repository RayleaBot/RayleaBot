package pluginapi

const (
	codeInvalidRequest  = "platform.invalid_request"
	codeResourceMissing = "platform.resource_missing"
)

type taskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}
