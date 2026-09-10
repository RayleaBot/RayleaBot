package desktop

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed api-contracts/responses.generated.json
var serverResponseSchemas []byte
var serverSchemaOnce sync.Once
var serverSchemas map[string]*jsonschema.Schema
var serverSchemaError error

const maxManagementResponseBytes = 4 << 20

type BoundaryError struct {
	Code, Message string
	Cause         error
}

func (e *BoundaryError) Error() string { return e.Code + ": " + e.Message }
func (e *BoundaryError) Unwrap() error { return e.Cause }

func decodeServerResponse[T any](reader io.Reader, name string) (*T, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxManagementResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxManagementResponseBytes {
		return nil, &BoundaryError{Code: "launcher.response_too_large", Message: "服务响应超出大小限制。"}
	}
	serverSchemaOnce.Do(func() {
		var document map[string]any
		if serverSchemaError = json.Unmarshal(serverResponseSchemas, &document); serverSchemaError != nil {
			return
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		const location = "urn:rayleabot:launcher-responses"
		if serverSchemaError = compiler.AddResource(location, document); serverSchemaError != nil {
			return
		}
		serverSchemas = make(map[string]*jsonschema.Schema)
		for name := range document["$defs"].(map[string]any) {
			schema, err := compiler.Compile(location + "#/$defs/" + name)
			if err != nil {
				serverSchemaError = err
				return
			}
			serverSchemas[name] = schema
		}
	})
	if serverSchemaError != nil {
		return nil, fmt.Errorf("compile Launcher response schema: %w", serverSchemaError)
	}
	schema, exists := serverSchemas[name]
	if !exists {
		return nil, fmt.Errorf("unknown Launcher response schema %q", name)
	}
	fail := func(cause error) (*T, error) {
		return nil, &BoundaryError{Code: "launcher.invalid_server_response", Message: "服务响应不符合正式接口约定。", Cause: cause}
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fail(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fail(fmt.Errorf("multiple JSON values"))
	}
	if err := schema.Validate(value); err != nil {
		return fail(err)
	}
	var result T
	if err := json.Unmarshal(payload, &result); err != nil {
		return fail(err)
	}
	return &result, nil
}
