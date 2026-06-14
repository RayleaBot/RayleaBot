package system

import "errors"

const (
	codeInvalidRequest  = "platform.invalid_request"
	codeResourceMissing = "platform.resource_missing"
)

var errSystemTaskUnavailable = errors.New("system task service unavailable")
