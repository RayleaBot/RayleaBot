package localaction

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
)

func (s *Service) executeStorageKV(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "storage.kv") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "storage.kv capability is not granted",
		}
	}
	if s.pluginKV == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "storage.kv repository is not available",
		}
	}

	switch action.StorageOperation {
	case "get":
		value, exists, err := s.pluginKV.Get(ctx, pluginID, action.StorageKey)
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.kv get failed", Err: err}
		}
		result := map[string]any{
			"key":    action.StorageKey,
			"exists": exists,
		}
		if exists {
			result["value"] = value
		}
		return result, nil
	case "set":
		err := s.pluginKV.Set(ctx, pluginID, action.StorageKey, action.StorageValue, currentKVLimits(s.config()))
		if errors.Is(err, pluginkv.ErrValueTooLarge) || errors.Is(err, pluginkv.ErrQuotaExceeded) {
			return nil, &runtimemanager.Error{Code: "platform.value_too_large", Message: "storage.kv value exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.kv set failed", Err: err}
		}
		return map[string]any{}, nil
	case "delete":
		deleted, err := s.pluginKV.Delete(ctx, pluginID, action.StorageKey)
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.kv delete failed", Err: err}
		}
		return map[string]any{
			"key":     action.StorageKey,
			"deleted": deleted,
		}, nil
	case "list":
		keys, err := s.pluginKV.List(ctx, pluginID, action.StoragePrefix)
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.kv list failed", Err: err}
		}
		return map[string]any{
			"prefix": action.StoragePrefix,
			"keys":   keys,
		}, nil
	default:
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported storage.kv operation",
		}
	}
}
