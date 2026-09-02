package actions

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
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
			metadata: Metadata{
				Action:         "storage.kv",
				Permission:     "storage.kv",
				RequestSchema:  "plugin-protocol.action_storage_kv",
				ResponseSchema: "plugin-protocol.local_action_result",
				AuditFields:    []string{"plugin_id", "operation", "key", "prefix"},
				ErrorCodes:     commonErrorCodes("platform.value_too_large"),
			},
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeStorageKV(ctx, deps, req)
				}
			},
		},
		{
			metadata: Metadata{
				Action:         "storage.file",
				Permission:     "storage.file",
				RequestSchema:  "plugin-protocol.action_storage_file",
				ResponseSchema: "plugin-protocol.local_action_result",
				WritesFile:     true,
				AuditFields:    []string{"plugin_id", "operation", "root", "path", "prefix"},
				ErrorCodes:     commonErrorCodes("platform.invalid_request", "platform.value_too_large"),
			},
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeStorageFile(ctx, deps, req)
				}
			},
		},
	}
}

func executeStorageKV(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.PluginKV == nil {
		return nil, &pluginruntime.Error{
			Code:    "plugin.internal_error",
			Message: "storage.kv repository is not available",
		}
	}

	switch req.Action.StorageOperation {
	case "get":
		value, exists, err := deps.PluginKV.Get(ctx, req.PluginID, req.Action.StorageKey)
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "storage.kv get failed", Err: err}
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
			return nil, &pluginruntime.Error{Code: "platform.value_too_large", Message: "storage.kv value exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "storage.kv set failed", Err: err}
		}
		return map[string]any{}, nil
	case "delete":
		deleted, err := deps.PluginKV.Delete(ctx, req.PluginID, req.Action.StorageKey)
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "storage.kv delete failed", Err: err}
		}
		return map[string]any{
			"key":     req.Action.StorageKey,
			"deleted": deleted,
		}, nil
	case "list":
		keys, err := deps.PluginKV.List(ctx, req.PluginID, req.Action.StoragePrefix)
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "storage.kv list failed", Err: err}
		}
		return map[string]any{
			"prefix": req.Action.StoragePrefix,
			"keys":   keys,
		}, nil
	default:
		return nil, &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported storage.kv operation",
		}
	}
}

func executeStorageFile(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.PluginFiles == nil {
		return nil, &pluginruntime.Error{
			Code:    "plugin.internal_error",
			Message: "storage.file service is not available",
		}
	}

	switch req.Action.StorageOperation {
	case "read":
		result, err := deps.PluginFiles.Read(req.PluginID, req.Action.StoragePath)
		if errors.Is(err, pluginstore.ErrFileInvalidPath) {
			return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "storage.file read failed", Err: err}
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
			return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if errors.Is(err, pluginstore.ErrFileTooLarge) {
			return nil, &pluginruntime.Error{Code: "platform.value_too_large", Message: "storage.file write exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "storage.file write failed", Err: err}
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
			deps.Logger.Warn(fmt.Sprintf("插件 %s 的文件工作目录已使用 %d 字节，超过 %d 字节软限制；本次写入已完成，建议清理旧文件。", req.PluginID, writeResult.UsageBytes, writeResult.SoftLimitBytes),
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
			return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "storage.file delete failed", Err: err}
		}
		return map[string]any{
			"root":    "plugin_data",
			"path":    req.Action.StoragePath,
			"deleted": deleted,
		}, nil
	case "list":
		paths, err := deps.PluginFiles.List(req.PluginID, req.Action.StoragePrefix)
		if errors.Is(err, pluginstore.ErrFileInvalidPath) {
			return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "storage.file list failed", Err: err}
		}
		return map[string]any{
			"root":   "plugin_data",
			"prefix": req.Action.StoragePrefix,
			"paths":  paths,
		}, nil
	default:
		return nil, &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
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
