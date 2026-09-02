package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func TestExecuteStorageFileRoundTripUsesImplicitPrivateNamespace(t *testing.T) {
	t.Parallel()
	application := newTestAppState(config.Config{Storage: config.StorageConfig{FileMaxBytes: 1024, PluginWorkDirSoftLimitMB: 1}}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubPermissionView{permissions: map[string][]stubPermission{}}, nil,
		pluginstore.NewFileService(filepath.Join(t.TempDir(), "plugins")), nil, nil, nil, nil, nil, nil, nil,
	)
	writeResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_1", pluginruntime.Action{
		Kind: "storage.file", StorageOperation: "write", StoragePath: "cache/example.txt", StorageContent: []byte("hello file"),
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if writeResult["soft_limit_exceeded"] != false {
		t.Fatalf("write result = %#v", writeResult)
	}
	readResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_2", pluginruntime.Action{
		Kind: "storage.file", StorageOperation: "read", StoragePath: "cache/example.txt",
	})
	if err != nil || readResult["content_text"] != "hello file" {
		t.Fatalf("read result = %#v, err = %v", readResult, err)
	}
	_, err = application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_3", pluginruntime.Action{
		Kind: "storage.file", StorageOperation: "write", StoragePath: "cache/blob.bin", StorageContent: []byte{0xff, 0x00, 0x01},
	})
	if err != nil {
		t.Fatalf("binary write: %v", err)
	}
	binaryResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_4", pluginruntime.Action{
		Kind: "storage.file", StorageOperation: "read", StoragePath: "cache/blob.bin",
	})
	if err != nil || binaryResult["content_base64"] != base64.StdEncoding.EncodeToString([]byte{0xff, 0x00, 0x01}) {
		t.Fatalf("binary read = %#v, err = %v", binaryResult, err)
	}
}

func TestExecuteStorageFileNamespacesPlugins(t *testing.T) {
	t.Parallel()
	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubPermissionView{permissions: map[string][]stubPermission{}}, nil,
		pluginstore.NewFileService(filepath.Join(t.TempDir(), "plugins")), nil, nil, nil, nil, nil, nil, nil,
	)
	_, err := application.executeLocalAction(context.Background(), "first", "req_first", pluginruntime.Action{
		Kind: "storage.file", StorageOperation: "write", StoragePath: "value.txt", StorageContent: []byte("private"),
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := application.executeLocalAction(context.Background(), "second", "req_second", pluginruntime.Action{
		Kind: "storage.file", StorageOperation: "read", StoragePath: "value.txt",
	})
	if err != nil || result["exists"] != false {
		t.Fatalf("cross-plugin namespace was not isolated: %#v, err = %v", result, err)
	}
}
