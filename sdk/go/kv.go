package rayleabot

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/sdk/go/internal/pluginwire"
)

type KVSetOptions struct {
	// TTL is an exact number of seconds, up to one year. Zero writes permanently.
	TTL time.Duration
}

type KVSetResult struct {
	ExpiresAtMS *int64 `json:"expires_at_ms,omitempty"`
}

func (actions *Actions) KVSetWithOptions(ctx context.Context, key string, value any, options KVSetOptions) (KVSetResult, error) {
	if options.TTL < 0 || options.TTL%time.Second != 0 || options.TTL > 31536000*time.Second {
		return KVSetResult{}, errors.New("rayleabot: KV TTL must be zero or a whole number of seconds up to 31536000")
	}
	request := struct {
		Operation  string `json:"operation"`
		Key        string `json:"key"`
		Value      any    `json:"value"`
		TTLSeconds int64  `json:"ttl_seconds,omitempty"`
	}{Operation: "set", Key: key, Value: value, TTLSeconds: int64(options.TTL / time.Second)}
	var response json.RawMessage
	if err := actions.Call(ctx, "storage.kv", request, &response); err != nil {
		return KVSetResult{}, err
	}
	if err := pluginwire.ValidateActionResult("storage.kv.set", response); err != nil {
		return KVSetResult{}, err
	}
	var result KVSetResult
	if err := json.Unmarshal(response, &result); err != nil {
		return KVSetResult{}, err
	}
	if (options.TTL > 0) != (result.ExpiresAtMS != nil) {
		return KVSetResult{}, errors.New("rayleabot: KV response expiry does not match the requested lifetime")
	}
	return result, nil
}
