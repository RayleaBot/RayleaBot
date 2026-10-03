package runtime

import (
	"bufio"
	"bytes"
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
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		contentBytes := len(fragment)
		if len(fragment) > 0 && fragment[len(fragment)-1] == '\n' {
			contentBytes--
		}
		if len(line)+contentBytes > maxBytes {
			return nil, fmt.Errorf("%w: limit %d bytes", errProtocolFrameTooLarge, maxBytes)
		}
		if line == nil {
			if err == nil {
				return bytes.Clone(fragment), nil
			}
			if !errors.Is(err, bufio.ErrBufferFull) {
				return nil, err
			}
			line = make([]byte, 0, min(maxBytes, 64*1024))
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
