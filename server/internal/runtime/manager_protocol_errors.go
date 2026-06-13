package runtime

import (
	"errors"
	"io"
	"os"
	"strings"
)

func isIgnorableShutdownWriteError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, os.ErrClosed) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "broken pipe") || strings.Contains(message, "pipe is being closed")
}

func classifyProtocolReadError(handle *processHandle, readErr error, exitMessage string, protocolMessage string) *Error {
	if waitErr, exited := handle.exitResult(); exited {
		if waitErr == nil {
			return errorf(codePluginInternalError, exitMessage, nil)
		}
		return errorf(codePluginInternalError, exitMessage, waitErr)
	}
	if isProcessPipeClosedError(readErr) {
		return errorf(codePluginInternalError, exitMessage, nil)
	}
	return errorf(codePluginProtocolViolation, protocolMessage, readErr)
}

func isProcessPipeClosedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "file already closed") || strings.Contains(message, "bad file descriptor")
}
