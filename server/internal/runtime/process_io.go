package runtime

import (
	"encoding/json"
	"fmt"
	"io"
)

func (h *processHandle) writeJSONLine(value any) error {
	if h == nil {
		return fmt.Errorf("plugin process handle is not available")
	}

	h.writeMu.Lock()
	defer h.writeMu.Unlock()

	return writeJSONLine(h.stdin, value)
}

func writeJSONLine(writer io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if !json.Valid(encoded) {
		return fmt.Errorf("protocol frame encoded invalid json")
	}

	data := append(encoded, '\n')
	for len(data) > 0 {
		written, writeErr := writer.Write(data)
		if written > 0 {
			data = data[written:]
		}
		if writeErr != nil {
			return writeErr
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}

	return nil
}
