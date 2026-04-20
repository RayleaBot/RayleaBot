package localaction

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (s *Service) executeStorageKV(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "storage.kv") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "storage.kv capability is not granted",
		}
	}
	if s.pluginKV == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "storage.kv repository is not available",
		}
	}

	switch action.StorageOperation {
	case "get":
		value, exists, err := s.pluginKV.Get(ctx, pluginID, action.StorageKey)
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.kv get failed", Err: err}
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
			return nil, &runtime.Error{Code: "platform.value_too_large", Message: "storage.kv value exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.kv set failed", Err: err}
		}
		return map[string]any{}, nil
	case "delete":
		deleted, err := s.pluginKV.Delete(ctx, pluginID, action.StorageKey)
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.kv delete failed", Err: err}
		}
		return map[string]any{
			"key":     action.StorageKey,
			"deleted": deleted,
		}, nil
	case "list":
		keys, err := s.pluginKV.List(ctx, pluginID, action.StoragePrefix)
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.kv list failed", Err: err}
		}
		return map[string]any{
			"prefix": action.StoragePrefix,
			"keys":   keys,
		}, nil
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported storage.kv operation",
		}
	}
}

func (s *Service) executeStorageFile(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "storage.file") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "storage.file capability is not granted",
		}
	}
	if !s.grants.StorageRootGranted(ctx, pluginID, action.StorageRoot) {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "storage.file root is outside the granted scope",
		}
	}
	if s.pluginFiles == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "storage.file service is not available",
		}
	}

	switch action.StorageOperation {
	case "read":
		result, err := s.pluginFiles.Read(pluginID, action.StoragePath)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.file read failed", Err: err}
		}
		payload := map[string]any{
			"root":   action.StorageRoot,
			"path":   action.StoragePath,
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
		err := s.pluginFiles.Write(pluginID, action.StoragePath, action.StorageContent, currentFileLimits(s.config()))
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if errors.Is(err, pluginfile.ErrFileTooLarge) || errors.Is(err, pluginfile.ErrQuotaExceeded) {
			return nil, &runtime.Error{Code: "platform.value_too_large", Message: "storage.file write exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.file write failed", Err: err}
		}
		return map[string]any{
			"root": action.StorageRoot,
			"path": action.StoragePath,
		}, nil
	case "delete":
		deleted, err := s.pluginFiles.Delete(pluginID, action.StoragePath)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.file delete failed", Err: err}
		}
		return map[string]any{
			"root":    action.StorageRoot,
			"path":    action.StoragePath,
			"deleted": deleted,
		}, nil
	case "list":
		paths, err := s.pluginFiles.List(pluginID, action.StoragePrefix)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.file list failed", Err: err}
		}
		return map[string]any{
			"root":   action.StorageRoot,
			"prefix": action.StoragePrefix,
			"paths":  paths,
		}, nil
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported storage.file operation",
		}
	}
}
