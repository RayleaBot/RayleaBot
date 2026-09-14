package desktop

import (
	"encoding/json"
	"io"
)

const maxManagementResponseBytes = 4 << 20

type BoundaryError struct {
	Code, Message string
	Cause         error
}

func (e *BoundaryError) Error() string { return e.Code + ": " + e.Message }
func (e *BoundaryError) Unwrap() error { return e.Cause }

// decodeServerResponse reads a bounded response from the Server shipped in the
// same package. Unknown fields are ignored; only malformed JSON is rejected.
func decodeServerResponse[T any](reader io.Reader) (*T, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxManagementResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxManagementResponseBytes {
		return nil, &BoundaryError{Code: "launcher.response_too_large", Message: "服务响应超出大小限制。"}
	}
	var result T
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, &BoundaryError{Code: "launcher.invalid_server_response", Message: "服务响应不是有效的 JSON。", Cause: err}
	}
	return &result, nil
}
