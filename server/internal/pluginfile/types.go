package pluginfile

import "errors"

var (
	ErrInvalidPath   = errors.New("plugin file path is invalid")
	ErrFileTooLarge  = errors.New("plugin file exceeds configured single-file limit")
	ErrQuotaExceeded = errors.New("plugin file workspace exceeds configured total limit")
)

type Limits struct {
	FileMaxBytes  int
	TotalMaxBytes int
}

type ReadResult struct {
	Exists  bool
	Content []byte
	IsText  bool
}
