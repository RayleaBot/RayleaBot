package plugininstall

import "fmt"

type installTaskError struct {
	Code    string
	Message string
	Summary string
}

func (e *installTaskError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func installError(code, message, summary string) error {
	return &installTaskError{
		Code:    code,
		Message: message,
		Summary: summary,
	}
}
