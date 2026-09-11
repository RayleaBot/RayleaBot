package runtime

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

func validatePluginFrame(line []byte) error {
	// readProtocolLine already enforces the process's configured byte limit.
	return pluginwire.Validate(line, len(line))
}

var errProtocolFrameTooLarge = errors.New("plugin IPC frame exceeds configured size limit")

func readProtocolLine(reader *bufio.Reader, maxBytes int) ([]byte, error) {
	if reader == nil {
		return nil, errors.New("plugin IPC reader is required")
	}
	if maxBytes <= 0 {
		maxBytes = pluginwire.DefaultMaxFrameBytes
	}
	line := make([]byte, 0, min(maxBytes, 64*1024))
	for {
		fragment, err := reader.ReadSlice('\n')
		contentBytes := len(fragment)
		if len(fragment) > 0 && fragment[len(fragment)-1] == '\n' {
			contentBytes--
		}
		if len(line)+contentBytes > maxBytes {
			return nil, fmt.Errorf("%w: limit %d bytes", errProtocolFrameTooLarge, maxBytes)
		}
		line = append(line, fragment...)
		if err == nil {
			return line, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return nil, err
	}
}
