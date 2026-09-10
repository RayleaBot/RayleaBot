package actions_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
)

func TestExecuteStorageFileRoundTripUsesImplicitPrivateNamespace(t *testing.T) {
	t.Parallel()
	testConfig := config.Config{Storage: config.StorageConfig{FileMaxBytes: 1024, PluginWorkDirSoftLimitMB: 1}}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	deps.Permissions = &scopedPermissionView{permissions: map[string][]stubPermission{}}
	deps.PluginFiles = pluginstore.NewFileService(filepath.Join(t.TempDir(), "plugins"))
	application := localaction.New(deps)
	writeResult, err := application.Execute(context.Background(), "scope-cache", "req_local_file_1", plugins.Action{
		Kind: "storage.file", StorageOperation: "write", StoragePath: "cache/example.txt", StorageContent: []byte("hello file"),
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if writeResult["soft_limit_exceeded"] != false {
		t.Fatalf("write result = %#v", writeResult)
	}
	readResult, err := application.Execute(context.Background(), "scope-cache", "req_local_file_2", plugins.Action{
		Kind: "storage.file", StorageOperation: "read", StoragePath: "cache/example.txt",
	}, chatevent.Event{})
	if err != nil || readResult["content_text"] != "hello file" {
		t.Fatalf("read result = %#v, err = %v", readResult, err)
	}
	_, err = application.Execute(context.Background(), "scope-cache", "req_local_file_3", plugins.Action{
		Kind: "storage.file", StorageOperation: "write", StoragePath: "cache/blob.bin", StorageContent: []byte{0xff, 0x00, 0x01},
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("binary write: %v", err)
	}
	binaryResult, err := application.Execute(context.Background(), "scope-cache", "req_local_file_4", plugins.Action{
		Kind: "storage.file", StorageOperation: "read", StoragePath: "cache/blob.bin",
	}, chatevent.Event{})
	if err != nil || binaryResult["content_base64"] != base64.StdEncoding.EncodeToString([]byte{0xff, 0x00, 0x01}) {
		t.Fatalf("binary read = %#v, err = %v", binaryResult, err)
	}
}

func TestExecuteStorageFileNamespacesPlugins(t *testing.T) {
	t.Parallel()
	testConfig := config.Config{}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	deps.Permissions = &scopedPermissionView{permissions: map[string][]stubPermission{}}
	deps.PluginFiles = pluginstore.NewFileService(filepath.Join(t.TempDir(), "plugins"))
	application := localaction.New(deps)
	_, err := application.Execute(context.Background(), "first", "req_first", plugins.Action{
		Kind: "storage.file", StorageOperation: "write", StoragePath: "value.txt", StorageContent: []byte("private"),
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := application.Execute(context.Background(), "second", "req_second", plugins.Action{
		Kind: "storage.file", StorageOperation: "read", StoragePath: "value.txt",
	}, chatevent.Event{})
	if err != nil || result["exists"] != false {
		t.Fatalf("cross-plugin namespace was not isolated: %#v, err = %v", result, err)
	}
}
