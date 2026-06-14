package storageaction

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type Grants interface {
	CapabilityGranted(context.Context, string, string) bool
	StorageRootGranted(context.Context, string, string) bool
}

type KVRepository interface {
	Get(context.Context, string, string) (any, bool, error)
	Set(context.Context, string, string, any, pluginkv.Limits) error
	Delete(context.Context, string, string) (bool, error)
	List(context.Context, string, string) ([]string, error)
}

type FileStore interface {
	Read(string, string) (pluginfile.ReadResult, error)
	Write(string, string, []byte, pluginfile.Limits) error
	Delete(string, string) (bool, error)
	List(string, string) ([]string, error)
}

type Request struct {
	PluginID string
	Action   runtimeaction.Action
	Config   config.Config
	Grants   Grants
	KV       KVRepository
	Files    FileStore
}

func ExecuteKV(ctx context.Context, req Request) (map[string]any, error) {
	if req.Grants == nil || !req.Grants.CapabilityGranted(ctx, req.PluginID, "storage.kv") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "storage.kv capability is not granted",
		}
	}
	if req.KV == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "storage.kv repository is not available",
		}
	}

	switch req.Action.StorageOperation {
	case "get":
		value, exists, err := req.KV.Get(ctx, req.PluginID, req.Action.StorageKey)
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.kv get failed", Err: err}
		}
		result := map[string]any{
			"key":    req.Action.StorageKey,
			"exists": exists,
		}
		if exists {
			result["value"] = value
		}
		return result, nil
	case "set":
		err := req.KV.Set(ctx, req.PluginID, req.Action.StorageKey, req.Action.StorageValue, currentKVLimits(req.Config))
		if errors.Is(err, pluginkv.ErrValueTooLarge) || errors.Is(err, pluginkv.ErrQuotaExceeded) {
			return nil, &runtimemanager.Error{Code: "platform.value_too_large", Message: "storage.kv value exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.kv set failed", Err: err}
		}
		return map[string]any{}, nil
	case "delete":
		deleted, err := req.KV.Delete(ctx, req.PluginID, req.Action.StorageKey)
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.kv delete failed", Err: err}
		}
		return map[string]any{
			"key":     req.Action.StorageKey,
			"deleted": deleted,
		}, nil
	case "list":
		keys, err := req.KV.List(ctx, req.PluginID, req.Action.StoragePrefix)
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.kv list failed", Err: err}
		}
		return map[string]any{
			"prefix": req.Action.StoragePrefix,
			"keys":   keys,
		}, nil
	default:
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported storage.kv operation",
		}
	}
}
