package runtime

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
)

func validatePluginFrameV2(line []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(line, &object); err != nil {
		return err
	}
	for _, removed := range []string{"protocol_version", "plugin_id", "timestamp", "subscriptions"} {
		if _, exists := object[removed]; exists {
			return fmt.Errorf("removed protocol v1 field %q is not allowed", removed)
		}
	}
	return nil
}

var errProtocolFrameTooLarge = errors.New("plugin IPC frame exceeds configured size limit")

func readProtocolLine(reader *bufio.Reader, maxBytes int) ([]byte, error) {
	if reader == nil {
		return nil, errors.New("plugin IPC reader is required")
	}
	if maxBytes <= 0 {
		maxBytes = 8 * 1024 * 1024
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
