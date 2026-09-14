package actions

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
)

const (
	defaultKVValueMaxBytes      = 65536
	defaultKVTotalLimitMegabyte = 16
)

func storageRegistrars() []registrar {
	return []registrar{
		{
			kind: "storage.kv",
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeStorageKV(ctx, deps, req)
				}
			},
		},
	}
}

func executeStorageKV(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.PluginKV == nil {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginInternalError,
			Message: "storage.kv repository is not available",
		}
	}

	switch req.Action.StorageOperation {
	case "get":
		entry, err := deps.PluginKV.GetEntry(ctx, req.PluginID, req.Action.StorageKey)
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.kv get failed", Err: err}
		}
		result := map[string]any{
			"key":    req.Action.StorageKey,
			"exists": entry.Exists,
		}
		if entry.Exists {
			result["value"] = entry.Value
			if entry.ExpiresAtMS != nil {
				result["expires_at_ms"] = *entry.ExpiresAtMS
			}
		}
		return result, nil
	case "set":
		outcome, err := deps.PluginKV.SetWithOptions(ctx, req.PluginID, req.Action.StorageKey, req.Action.StorageValue, currentKVLimits(currentConfig(deps)), pluginstore.KVSetOptions{TTLSeconds: req.Action.StorageTTLSeconds})
		if errors.Is(err, pluginstore.ErrKVInvalidRequest) {
			return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "storage.kv parameters are invalid"}
		}
		if errors.Is(err, pluginstore.ErrKVValueTooLarge) || errors.Is(err, pluginstore.ErrKVQuotaExceeded) {
			return nil, &plugins.Error{Code: errorcodes.PlatformValueTooLarge, Message: "storage.kv value exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.kv set failed", Err: err}
		}
		result := map[string]any{}
		if outcome.ExpiresAtMS != nil {
			result["expires_at_ms"] = *outcome.ExpiresAtMS
		}
		return result, nil
	case "delete":
		deleted, err := deps.PluginKV.Delete(ctx, req.PluginID, req.Action.StorageKey)
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.kv delete failed", Err: err}
		}
		return map[string]any{
			"key":     req.Action.StorageKey,
			"deleted": deleted,
		}, nil
	case "list":
		keys, err := deps.PluginKV.List(ctx, req.PluginID, req.Action.StoragePrefix)
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.kv list failed", Err: err}
		}
		return map[string]any{
			"prefix": req.Action.StoragePrefix,
			"keys":   keys,
		}, nil
	default:
		return nil, &plugins.Error{
			Code:    errorcodes.PluginProtocolViolation,
			Message: "received unsupported storage.kv operation",
		}
	}
}

func currentKVLimits(cfg config.Config) pluginstore.KVLimits {
	valueLimit := cfg.Storage.KVValueMaxBytes
	if valueLimit <= 0 {
		valueLimit = defaultKVValueMaxBytes
	}
	totalLimitMB := cfg.Storage.KVTotalLimitMB
	if totalLimitMB <= 0 {
		totalLimitMB = defaultKVTotalLimitMegabyte
	}
	return pluginstore.KVLimits{
		ValueMaxBytes: valueLimit,
		TotalMaxBytes: totalLimitMB * 1024 * 1024,
	}
}
