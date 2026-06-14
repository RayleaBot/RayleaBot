package storageaction

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
)

func ExecuteFile(ctx context.Context, req Request) (map[string]any, error) {
	if req.Grants == nil || !req.Grants.CapabilityGranted(ctx, req.PluginID, "storage.file") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "storage.file capability is not granted",
		}
	}
	if !req.Grants.StorageRootGranted(ctx, req.PluginID, req.Action.StorageRoot) {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "storage.file root is outside the granted scope",
		}
	}
	if req.Files == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "storage.file service is not available",
		}
	}

	switch req.Action.StorageOperation {
	case "read":
		result, err := req.Files.Read(req.PluginID, req.Action.StoragePath)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtimemanager.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.file read failed", Err: err}
		}
		payload := map[string]any{
			"root":   req.Action.StorageRoot,
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
		err := req.Files.Write(req.PluginID, req.Action.StoragePath, req.Action.StorageContent, currentFileLimits(req.Config))
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtimemanager.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if errors.Is(err, pluginfile.ErrFileTooLarge) || errors.Is(err, pluginfile.ErrQuotaExceeded) {
			return nil, &runtimemanager.Error{Code: "platform.value_too_large", Message: "storage.file write exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.file write failed", Err: err}
		}
		return map[string]any{
			"root": req.Action.StorageRoot,
			"path": req.Action.StoragePath,
		}, nil
	case "delete":
		deleted, err := req.Files.Delete(req.PluginID, req.Action.StoragePath)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtimemanager.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.file delete failed", Err: err}
		}
		return map[string]any{
			"root":    req.Action.StorageRoot,
			"path":    req.Action.StoragePath,
			"deleted": deleted,
		}, nil
	case "list":
		paths, err := req.Files.List(req.PluginID, req.Action.StoragePrefix)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtimemanager.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "storage.file list failed", Err: err}
		}
		return map[string]any{
			"root":   req.Action.StorageRoot,
			"prefix": req.Action.StoragePrefix,
			"paths":  paths,
		}, nil
	default:
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported storage.file operation",
		}
	}
}
