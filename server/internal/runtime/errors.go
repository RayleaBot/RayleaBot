package runtime

import "fmt"

const (
	codePlatformInvalidRequest  = "platform.invalid_request"
	codePlatformResourceMissing = "platform.resource_missing"
	codePluginInitTimeout       = "plugin.init_timeout"
	codePluginEventTimeout      = "plugin.event_timeout"
	codePluginInternalError     = "plugin.internal_error"
	codePluginNotHandled        = "plugin.not_handled"
	codePluginProtocolViolation = "plugin.protocol_violation"
	codePluginShutdownTimeout   = "plugin.shutdown_timeout"
	codePluginStopping          = "plugin.stopping"
)

type Error struct {
	Code    string
	Message string
	Details map[string]any
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func errorf(code, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func errorWithDetails(code, message string, details map[string]any, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: cloneDetails(details),
		Err:     err,
	}
}

func cloneDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[key] = value
	}
	return cloned
}
