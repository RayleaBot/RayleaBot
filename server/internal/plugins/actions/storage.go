package actions

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
)

const (
	defaultKVValueMaxBytes      = 65536
	defaultKVTotalLimitMegabyte = 16
	defaultFileMaxBytes         = 10 * 1024 * 1024
	defaultPluginWorkdirMB      = 256
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
		{
			kind: "storage.file",
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeStorageFile(deps, req)
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
		value, exists, err := deps.PluginKV.Get(ctx, req.PluginID, req.Action.StorageKey)
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.kv get failed", Err: err}
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
		err := deps.PluginKV.Set(ctx, req.PluginID, req.Action.StorageKey, req.Action.StorageValue, currentKVLimits(currentConfig(deps)))
		if errors.Is(err, pluginstore.ErrKVValueTooLarge) || errors.Is(err, pluginstore.ErrKVQuotaExceeded) {
			return nil, &plugins.Error{Code: errorcodes.PlatformValueTooLarge, Message: "storage.kv value exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.kv set failed", Err: err}
		}
		return map[string]any{}, nil
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

func executeStorageFile(deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.PluginFiles == nil {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginInternalError,
			Message: "storage.file service is not available",
		}
	}

	switch req.Action.StorageOperation {
	case "read":
		result, err := deps.PluginFiles.Read(req.PluginID, req.Action.StoragePath)
		if errors.Is(err, pluginstore.ErrFileInvalidPath) {
			return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.file read failed", Err: err}
		}
		payload := map[string]any{
			"root":   "plugin_data",
			"path":   req.Action.StoragePath,
			"exists": result.Exists,
		}
		if result.Exists {
			if result.IsText {
				payload["content_text"] = string(result.Content)
			} else {
				payload["content_base64"] = base64.StdEncoding.EncodeToString(result.Content)
			}
		}
		return payload, nil
	case "write":
		writeResult, err := deps.PluginFiles.WriteWithResult(req.PluginID, req.Action.StoragePath, req.Action.StorageContent, currentFileLimits(currentConfig(deps)))
		if errors.Is(err, pluginstore.ErrFileInvalidPath) {
			return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "storage.file path is invalid"}
		}
		if errors.Is(err, pluginstore.ErrFileTooLarge) {
			return nil, &plugins.Error{Code: errorcodes.PlatformValueTooLarge, Message: "storage.file write exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.file write failed", Err: err}
		}
		result := map[string]any{
			"root":                "plugin_data",
			"path":                req.Action.StoragePath,
			"usage_bytes":         writeResult.UsageBytes,
			"soft_limit_bytes":    writeResult.SoftLimitBytes,
			"soft_limit_exceeded": writeResult.SoftLimitExceeded,
			"cleanup_recommended": writeResult.SoftLimitExceeded,
		}
		if writeResult.SoftLimitExceeded && deps.Logger != nil {
			deps.Logger.Warn("插件文件工作目录超过软限制；本次写入已完成，建议清理旧文件",
				"component", "plugin_action",
				"plugin_id", req.PluginID,
				"usage_bytes", writeResult.UsageBytes,
				"soft_limit_bytes", writeResult.SoftLimitBytes,
			)
		}
		return result, nil
	case "delete":
		deleted, err := deps.PluginFiles.Delete(req.PluginID, req.Action.StoragePath)
		if errors.Is(err, pluginstore.ErrFileInvalidPath) {
			return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.file delete failed", Err: err}
		}
		return map[string]any{
			"root":    "plugin_data",
			"path":    req.Action.StoragePath,
			"deleted": deleted,
		}, nil
	case "list":
		paths, err := deps.PluginFiles.List(req.PluginID, req.Action.StoragePrefix)
		if errors.Is(err, pluginstore.ErrFileInvalidPath) {
			return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "storage.file list failed", Err: err}
		}
		return map[string]any{
			"root":   "plugin_data",
			"prefix": req.Action.StoragePrefix,
			"paths":  paths,
		}, nil
	default:
		return nil, &plugins.Error{
			Code:    errorcodes.PluginProtocolViolation,
			Message: "received unsupported storage.file operation",
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

func currentFileLimits(cfg config.Config) pluginstore.FileLimits {
	fileLimit := cfg.Storage.FileMaxBytes
	if fileLimit <= 0 {
		fileLimit = defaultFileMaxBytes
	}
	totalLimitMB := cfg.Storage.PluginWorkDirSoftLimitMB
	if totalLimitMB <= 0 {
		totalLimitMB = defaultPluginWorkdirMB
	}
	return pluginstore.FileLimits{
		FileMaxBytes:   fileLimit,
		SoftLimitBytes: totalLimitMB * 1024 * 1024,
	}
}
