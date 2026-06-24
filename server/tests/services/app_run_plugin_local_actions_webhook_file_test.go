package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	"log/slog"
	"path/filepath"
	"testing"
)

func TestExecuteExposeWebhookRegistersGateway(t *testing.T) {
	t.Parallel()

	capabilityRepo := &stubCapabilityView{
		capabilities: map[string][]stubCapability{
			"repo-watcher": {{
				PluginID:   "repo-watcher",
				Capability: "event.expose_webhook",
				ScopeJSON:  `{"webhooks":[{"route":"github","auth_strategy":"hmac_sha256","header":"X-Hub-Signature-256","secret_ref":"webhook.github.secret","source_ips":["192.0.2.0/24"]}]}`,
			}},
		},
	}
	application := newTestAppState(config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	registry := newPluginWebhookRegistry()
	application.setTestLocalActions(capabilityRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	application.setTestWebhookService(nil, nil, nil, registry)

	result, err := application.executeLocalAction(context.Background(), "repo-watcher", "req_webhook_1", runtimeaction.Action{
		Kind:                   "event.expose_webhook",
		WebhookRoute:           "github",
		WebhookMethods:         []string{"POST"},
		WebhookAuthStrategy:    "hmac_sha256",
		WebhookHeader:          "X-Hub-Signature-256",
		WebhookSecretRef:       "webhook.github.secret",
		WebhookSignaturePrefix: "sha256=",
		WebhookReplayProtection: &runtimeaction.WebhookReplayProtection{
			TimestampHeader:  "X-Raylea-Timestamp",
			EventIDHeader:    "X-Raylea-Event-Id",
			ToleranceSeconds: 300,
			Enforce:          true,
		},
	})
	if err != nil {
		t.Fatalf("event.expose_webhook failed: %v", err)
	}
	if result["route"] != "github" {
		t.Fatalf("unexpected route result: %#v", result)
	}
	urlValue, _ := result["url"].(string)
	if urlValue != "http://127.0.0.1:8080/api/webhooks/repo-watcher/github" {
		t.Fatalf("unexpected webhook url: %#v", urlValue)
	}

	registration, ok := application.pluginStack.Webhooks.Get("repo-watcher", "github")
	if !ok {
		t.Fatal("expected webhook registration to be stored")
	}
	if registration.AuthStrategy != "hmac_sha256" || registration.SecretRef != "webhook.github.secret" {
		t.Fatalf("unexpected webhook registration: %#v", registration)
	}
	if len(registration.SourceIPs) != 1 || registration.SourceIPs[0] != "192.0.2.0/24" {
		t.Fatalf("unexpected webhook source IPs: %#v", registration.SourceIPs)
	}
}

func TestExecuteStorageFileRoundTrip(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{
		Storage: config.StorageConfig{
			FileMaxBytes:    1024,
			PluginWorkDirMB: 1,
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubCapabilityView{
			capabilities: map[string][]stubCapability{
				"scope-cache": {{
					PluginID:   "scope-cache",
					Capability: "storage.file",
					ScopeJSON:  `{"storage_roots":["plugin_data"]}`,
				}},
			},
		},
		nil,
		pluginfile.NewService(filepath.Join(t.TempDir(), "plugins")),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if _, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_1", runtimeaction.Action{
		Kind:             "storage.file",
		StorageOperation: "write",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
		StorageContent:   []byte("hello file"),
	}); err != nil {
		t.Fatalf("storage.file write failed: %v", err)
	}

	readResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_2", runtimeaction.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
	})
	if err != nil {
		t.Fatalf("storage.file read failed: %v", err)
	}
	if got := readResult["content_text"]; got != "hello file" {
		t.Fatalf("unexpected text content: %#v", got)
	}

	if _, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_3", runtimeaction.Action{
		Kind:             "storage.file",
		StorageOperation: "write",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/blob.bin",
		StorageContent:   []byte{0xff, 0x00, 0x01},
	}); err != nil {
		t.Fatalf("storage.file binary write failed: %v", err)
	}

	binaryResult, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_4", runtimeaction.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/blob.bin",
	})
	if err != nil {
		t.Fatalf("storage.file binary read failed: %v", err)
	}
	if got := binaryResult["content_base64"]; got != base64.StdEncoding.EncodeToString([]byte{0xff, 0x00, 0x01}) {
		t.Fatalf("unexpected base64 content: %#v", got)
	}
}

func TestExecuteStorageFileRejectsMissingScope(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubCapabilityView{
			capabilities: map[string][]stubCapability{
				"scope-cache": {{
					PluginID:   "scope-cache",
					Capability: "storage.file",
					ScopeJSON:  `{"storage_roots":[]}`,
				}},
			},
		},
		nil,
		pluginfile.NewService(filepath.Join(t.TempDir(), "plugins")),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, err := application.executeLocalAction(context.Background(), "scope-cache", "req_local_file_5", runtimeaction.Action{
		Kind:             "storage.file",
		StorageOperation: "read",
		StorageRoot:      "plugin_data",
		StoragePath:      "cache/example.txt",
	})
	assertRuntimeErrorCode(t, err, "plugin.capability_violation")
}
