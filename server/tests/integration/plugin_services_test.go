package integration

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestPluginServicesAcrossNativeSDKProcesses(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var registry *pluginruntime.Registry
	var kvMu sync.Mutex
	stored := make(map[string]map[string]any)
	waiting := make(chan struct{}, 8)
	peerEntered, peerCalls := make(chan string, 4), make(chan string, 4)
	releasePeers := make(chan struct{})
	actions := localaction.New(localaction.Deps{CallService: func(ctx context.Context, caller string, request plugins.ServiceCall, origin chatevent.Event) (map[string]any, error) {
		return registry.CallService(ctx, caller, request, origin)
	}})
	execute := func(ctx context.Context, pluginID, requestID string, action plugins.Action, origin chatevent.Event) (map[string]any, error) {
		if action.Kind != "storage.kv" {
			if action.Kind == "plugin.call" && strings.HasPrefix(pluginID, "peer-") {
				select {
				case peerCalls <- pluginID:
				default:
				}
			}
			return actions.Execute(ctx, pluginID, requestID, action, origin)
		}
		kvMu.Lock()
		if stored[pluginID] == nil {
			stored[pluginID] = make(map[string]any)
		}
		stored[pluginID][action.StorageKey] = action.StorageValue
		kvMu.Unlock()
		if strings.HasPrefix(pluginID, "peer-") && action.StorageKey == "last_requested_id" {
			select {
			case peerEntered <- pluginID:
			default:
			}
			select {
			case <-releasePeers:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if action.StorageKey == "waiting" {
			waiting <- struct{}{}
		}
		return map[string]any{}, nil
	}
	registry = pluginruntime.NewRegistry(logger, pluginruntime.Options{ExecuteLocalAction: execute})
	providerSpec := buildServiceExample(t, "example-service-provider")
	consumerSpec := buildServiceExample(t, "example-service-consumer")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := registry.StopAll(ctx); err != nil {
			t.Errorf("stop service runtimes: %v", err)
		}
	})
	start := func(spec pluginruntime.Spec) *pluginruntime.Manager {
		t.Helper()
		manager := registry.GetOrCreate(spec.PluginID)
		if err := manager.Start(t.Context(), spec, pluginruntime.InitPayload{Timezone: "UTC", CommandPrefixes: []string{"/"}, Config: map[string]any{"provider": providerSpec.PluginID}}); err != nil {
			t.Fatal(err)
		}
		return manager
	}
	provider := start(providerSpec)
	consumer := start(consumerSpec)
	origin := chatevent.Event{EventID: "fixture-origin", EventType: "management.action", SourceProtocol: "platform", SourceAdapter: "management.internal", BotID: "fixture-bot", Timestamp: time.Now().UnixMilli(), Actor: &chatevent.Actor{ID: "fixture-user"}}
	invoke := func(ctx context.Context, method string, timeoutMS int) (plugins.Delivery, error) {
		event := origin
		event.PayloadFields = map[string]any{"action": "query", "payload": map[string]any{"method": method, "timeout_ms": timeoutMS, "params": map[string]any{"id": "fixture-item", "caller_plugin_id": "ignored-body-identity"}}}
		return consumer.DeliverEvent(ctx, event)
	}
	assertCode := func(t *testing.T, err error, code string) {
		t.Helper()
		var failure *plugins.Error
		if !errors.As(err, &failure) || failure.Code != code {
			t.Fatalf("expected %s, got %v", code, err)
		}
	}
	t.Run("caller origin and provider private actions", func(t *testing.T) {
		delivery, err := invoke(t.Context(), "query", 0)
		if err != nil {
			t.Fatal(err)
		}
		if delivery.Result["caller"] != consumerSpec.PluginID || delivery.Result["actor_id"] != "fixture-user" || delivery.Result["bot_id"] != "fixture-bot" {
			t.Fatalf("origin was not forwarded: %#v", delivery.Result)
		}
		kvMu.Lock()
		defer kvMu.Unlock()
		if stored[providerSpec.PluginID]["last_id"] != "fixture-item" || stored[consumerSpec.PluginID]["last_requested_id"] != "fixture-item" || stored[consumerSpec.PluginID]["last_id"] != nil {
			t.Fatalf("private storage ownership changed: %#v", stored)
		}
	})
	t.Run("route errors and one hop", func(t *testing.T) {
		base := plugins.ServiceCall{TargetPluginID: providerSpec.PluginID, Service: "resource", ServiceVersion: 1, Method: "query", Params: map[string]any{}}
		for _, scenario := range []struct {
			code string
			edit func(*plugins.ServiceCall)
		}{
			{"plugin.service_unavailable", func(c *plugins.ServiceCall) { c.TargetPluginID = "missing" }},
			{"plugin.service_not_found", func(c *plugins.ServiceCall) { c.Service = "missing" }},
			{"plugin.service_version_unsupported", func(c *plugins.ServiceCall) { c.ServiceVersion = 2 }},
			{"plugin.method_not_found", func(c *plugins.ServiceCall) { c.Method = "missing" }},
			{"plugin.call_chain_rejected", func(c *plugins.ServiceCall) { c.TargetPluginID = consumerSpec.PluginID }},
		} {
			request := base
			scenario.edit(&request)
			_, err := registry.CallService(t.Context(), consumerSpec.PluginID, request, origin)
			assertCode(t, err, scenario.code)
		}
		_, err := invoke(t.Context(), "nested", 0)
		assertCode(t, err, "plugin.call_chain_rejected")
	})
	t.Run("provider error retains structured details", func(t *testing.T) {
		_, err := invoke(t.Context(), "fail", 0)
		assertCode(t, err, "plugin.service_unavailable")
		var failure *plugins.Error
		_ = errors.As(err, &failure)
		if failure.Details["reason"] != "fixture" {
			t.Fatalf("provider error details lost: %#v", failure.Details)
		}
	})
	t.Run("empty JSON values retain their meaning", func(t *testing.T) {
		params := map[string]any{"object": map[string]any{}, "array": []any{}, "null": nil, "large": json.Number("9007199254740993")}
		result, err := registry.CallService(t.Context(), consumerSpec.PluginID, plugins.ServiceCall{TargetPluginID: providerSpec.PluginID, Service: "resource", ServiceVersion: 1, Method: "query", Params: params}, origin)
		if err != nil {
			t.Fatal(err)
		}
		input, ok := result["input"].(map[string]any)
		if !ok {
			t.Fatal("missing echoed input")
		}
		if _, ok := input["object"].(map[string]any); !ok {
			t.Fatal("empty object changed to null")
		}
		if _, ok := input["array"].([]any); !ok || input["null"] != nil {
			t.Fatalf("empty array or null changed: %#v", input)
		}
		if input["large"] != json.Number("9007199254740993") {
			t.Fatalf("large integer changed: %v", input["large"])
		}
	})
	t.Run("oversized request does not crash provider", func(t *testing.T) {
		spec := providerSpec
		spec.PluginID = "small-provider"
		spec.IPCMessageMaxBytes = 4096
		manager := start(spec)
		request := plugins.ServiceCall{TargetPluginID: spec.PluginID, Service: "resource", ServiceVersion: 1, Method: "query", Params: map[string]any{"id": strings.Repeat("x", 5000)}}
		_, err := registry.CallService(t.Context(), consumerSpec.PluginID, request, origin)
		assertCode(t, err, "platform.value_too_large")
		request.Params = map[string]any{"id": "small"}
		if _, err := registry.CallService(t.Context(), consumerSpec.PluginID, request, origin); err != nil || manager.Snapshot().State != pluginruntime.StateRunning {
			t.Fatalf("oversized call damaged provider: %v", err)
		}
	})
	t.Run("abandoned call occupies the provider until its deadline", func(t *testing.T) {
		// The host sends no cancellation. A dedicated pair with a short provider
		// timeout keeps the shared provider free for the following scenarios.
		target := providerSpec
		target.PluginID = "deadline-provider"
		target.EventTimeout = time.Second
		targetManager := start(target)
		caller := consumerSpec
		caller.PluginID = "deadline-consumer"
		callerManager := registry.GetOrCreate(caller.PluginID)
		if err := callerManager.Start(t.Context(), caller, pluginruntime.InitPayload{Timezone: "UTC", CommandPrefixes: []string{"/"}, Config: map[string]any{"provider": target.PluginID}}); err != nil {
			t.Fatal(err)
		}
		call := func(method string, timeoutMS int) error {
			event := origin
			event.PayloadFields = map[string]any{"action": "query", "payload": map[string]any{"method": method, "timeout_ms": timeoutMS, "params": map[string]any{"id": "fixture-item"}}}
			_, err := callerManager.DeliverEvent(t.Context(), event)
			return err
		}
		assertCode(t, call("wait", 200), "plugin.event_canceled")
		select {
		case <-waiting:
		case <-time.After(time.Second):
			t.Fatal("wait handler did not start")
		}
		time.Sleep(target.EventTimeout)
		if err := call("query", 0); err != nil || targetManager.Snapshot().State != pluginruntime.StateRunning {
			t.Fatalf("provider did not recover after the abandoned call's deadline: state=%s err=%v", targetManager.Snapshot().State, err)
		}
	})
	t.Run("parent deadline releases caller and provider", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()
		_, err := invoke(ctx, "wait", 0)
		assertCode(t, err, "plugin.event_timeout")
		select {
		case <-waiting:
		case <-time.After(time.Second):
			t.Fatal("wait handler did not start")
		}
		next, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		if _, err := invoke(next, "query", 0); err != nil {
			t.Fatalf("parent deadline retained an SDK concurrency slot: %v", err)
		}
	})
	t.Run("simultaneous calls between serial peers cancel without deadlock", func(t *testing.T) {
		peers := make([]*pluginruntime.Manager, 2)
		for i, id := range []string{"peer-a", "peer-b"} {
			spec := consumerSpec
			spec.PluginID = id
			peers[i] = registry.GetOrCreate(id)
			target := "peer-a"
			if i == 0 {
				target = "peer-b"
			}
			if err := peers[i].Start(t.Context(), spec, pluginruntime.InitPayload{Timezone: "UTC", CommandPrefixes: []string{"/"}, Config: map[string]any{"provider": target}}); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		finished := make(chan error, 2)
		for _, peer := range peers {
			go func() { _, err := peer.DeliverEvent(ctx, origin); finished <- err }()
		}
		for range 2 {
			select {
			case <-peerEntered:
			case <-time.After(2 * time.Second):
				t.Fatal("peer did not enter ordinary handler")
			}
		}
		close(releasePeers)
		for range 2 {
			select {
			case <-peerCalls:
			case <-time.After(2 * time.Second):
				t.Fatal("peer did not issue its nested wait")
			}
		}
		cancel()
		for range 2 {
			select {
			case err := <-finished:
				assertCode(t, err, "plugin.event_canceled")
			case <-time.After(time.Second):
				t.Fatal("mutual calls did not settle")
			}
		}
		next, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		delivery, err := peers[0].DeliverEvent(next, origin)
		if err != nil || delivery.Result["owner"] != "peer-b" {
			t.Fatalf("peers did not release their SDK slots: %#v, %v", delivery.Result, err)
		}
	})
	t.Run("restart retires in-flight calls", func(t *testing.T) {
		finished := make(chan error, 1)
		go func() { _, err := invoke(t.Context(), "wait", 0); finished <- err }()
		select {
		case <-waiting:
		case <-time.After(time.Second):
			t.Fatal("wait handler did not start")
		}
		if err := provider.Stop(t.Context()); err != nil {
			t.Fatal(err)
		}
		provider = start(providerSpec)
		select {
		case err := <-finished:
			assertCode(t, err, "plugin.service_unavailable")
		case <-time.After(time.Second):
			t.Fatal("retired call did not settle")
		}
		if _, err := invoke(t.Context(), "query", 0); err != nil {
			t.Fatalf("new provider could not serve calls: %v", err)
		}
	})
	t.Run("stopped provider is not restarted", func(t *testing.T) {
		if err := provider.Stop(t.Context()); err != nil {
			t.Fatal(err)
		}
		_, err := invoke(t.Context(), "query", 0)
		assertCode(t, err, "plugin.service_unavailable")
		if provider.Snapshot().State != pluginruntime.StateStopped {
			t.Fatal("call restarted a stopped provider")
		}
	})
}

func buildServiceExample(t *testing.T, name string) pluginruntime.Spec {
	t.Helper()
	source := testutil.RepoPath(t, "examples", "plugins", name)
	directory := t.TempDir()
	filename := name
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	binary := filepath.Join(directory, filename)
	command := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./cmd/"+name)
	command.Dir = source
	command.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build service example: %v\n%s", err, output)
	}
	info, err := os.ReadFile(filepath.Join(source, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ID       string            `json:"id"`
		Services []plugins.Service `json:"services"`
	}
	if err := json.Unmarshal(info, &manifest); err != nil {
		t.Fatal(err)
	}
	return pluginruntime.Spec{PluginID: manifest.ID, Command: binary, WorkDir: directory, Services: manifest.Services, InitTimeout: 5 * time.Second, EventTimeout: 5 * time.Second, ShutdownGrace: time.Second, EffectiveConcurrency: 1, IPCMessageMaxBytes: 8 * 1024 * 1024, ValidateFrames: true}
}

// Each scenario owns its plugin instances, so an abandoned wait in one does not
// occupy a provider used by another.
func TestPluginServiceLifecycleEdges(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var registry *pluginruntime.Registry
	waiting := make(chan string, 8)
	actions := localaction.New(localaction.Deps{CallService: func(ctx context.Context, caller string, request plugins.ServiceCall, origin chatevent.Event) (map[string]any, error) {
		return registry.CallService(ctx, caller, request, origin)
	}})
	execute := func(ctx context.Context, pluginID, requestID string, action plugins.Action, origin chatevent.Event) (map[string]any, error) {
		if action.Kind != "storage.kv" {
			return actions.Execute(ctx, pluginID, requestID, action, origin)
		}
		if action.StorageKey == "waiting" {
			select {
			case waiting <- pluginID:
			default:
			}
		}
		return map[string]any{}, nil
	}
	registry = pluginruntime.NewRegistry(logger, pluginruntime.Options{ExecuteLocalAction: execute})
	providerSpec := buildServiceExample(t, "example-service-provider")
	consumerSpec := buildServiceExample(t, "example-service-consumer")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := registry.StopAll(ctx); err != nil {
			t.Errorf("stop service runtimes: %v", err)
		}
	})
	start := func(t *testing.T, spec pluginruntime.Spec, provider string) *pluginruntime.Manager {
		t.Helper()
		manager := registry.GetOrCreate(spec.PluginID)
		if err := manager.Start(t.Context(), spec, pluginruntime.InitPayload{Timezone: "UTC", CommandPrefixes: []string{"/"}, Config: map[string]any{"provider": provider}}); err != nil {
			t.Fatal(err)
		}
		return manager
	}
	origin := chatevent.Event{EventID: "fixture-origin", EventType: "management.action", SourceProtocol: "platform", SourceAdapter: "management.internal", BotID: "fixture-bot", Timestamp: time.Now().UnixMilli(), Actor: &chatevent.Actor{ID: "fixture-user"}}
	waitCall := func(target string) plugins.ServiceCall {
		return plugins.ServiceCall{TargetPluginID: target, Service: "resource", ServiceVersion: 1, Method: "wait", Params: map[string]any{}}
	}
	awaitWaiting := func(t *testing.T, pluginID string) {
		t.Helper()
		select {
		case started := <-waiting:
			if started != pluginID {
				t.Fatalf("wait handler started in %s, want %s", started, pluginID)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("wait handler did not start")
		}
	}
	assertCode := func(t *testing.T, err error, code string) *plugins.Error {
		t.Helper()
		var failure *plugins.Error
		if !errors.As(err, &failure) || failure.Code != code {
			t.Fatalf("expected %s, got %v", code, err)
		}
		return failure
	}

	t.Run("provider pending limit rejects further calls", func(t *testing.T) {
		spec := providerSpec
		spec.PluginID = "limit-provider"
		manager := start(t, spec, "")
		ctx, cancel := context.WithCancel(t.Context())
		var calls sync.WaitGroup
		for range 64 {
			calls.Go(func() { _, _ = registry.CallService(ctx, "limit-caller", waitCall(spec.PluginID), origin) })
		}
		awaitWaiting(t, spec.PluginID)
		// One handler runs; the rest queue inside the provider but are already
		// pending on the host, which is what the limit counts.
		time.Sleep(500 * time.Millisecond)
		probe, stop := context.WithTimeout(t.Context(), time.Second)
		_, err := registry.CallService(probe, "limit-caller", waitCall(spec.PluginID), origin)
		stop()
		if failure := assertCode(t, err, "platform.rate_limited"); failure.Details["limit"] != 64 {
			t.Fatalf("limit details = %#v", failure.Details)
		}
		cancel()
		calls.Wait()
		if manager.Snapshot().State != pluginruntime.StateRunning {
			t.Fatalf("provider state = %s", manager.Snapshot().State)
		}
	})
	t.Run("provider crash fails the in-flight call", func(t *testing.T) {
		spec := providerSpec
		spec.PluginID = "crash-provider"
		manager := start(t, spec, "")
		finished := make(chan error, 1)
		go func() {
			_, err := registry.CallService(t.Context(), "crash-caller", waitCall(spec.PluginID), origin)
			finished <- err
		}()
		awaitWaiting(t, spec.PluginID)
		process, err := os.FindProcess(manager.Snapshot().PID)
		if err != nil {
			t.Fatal(err)
		}
		if err := process.Kill(); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-finished:
			assertCode(t, err, "plugin.service_unavailable")
		case <-time.After(3 * time.Second):
			t.Fatal("call did not settle after the provider exited")
		}
	})
	t.Run("stopping the caller settles its outgoing call", func(t *testing.T) {
		target := providerSpec
		target.PluginID = "stop-provider"
		target.EventTimeout = time.Second
		targetManager := start(t, target, "")
		caller := consumerSpec
		caller.PluginID = "stop-consumer"
		callerManager := start(t, caller, target.PluginID)
		finished := make(chan error, 1)
		go func() {
			event := origin
			event.PayloadFields = map[string]any{"action": "query", "payload": map[string]any{"method": "wait", "params": map[string]any{"id": "fixture-item"}}}
			_, err := callerManager.DeliverEvent(t.Context(), event)
			finished <- err
		}()
		awaitWaiting(t, target.PluginID)
		stopCtx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		if err := callerManager.Stop(stopCtx); err != nil {
			t.Fatalf("caller did not stop while its service call was outstanding: %v", err)
		}
		select {
		case err := <-finished:
			assertCode(t, err, "plugin.event_canceled")
		case <-time.After(time.Second):
			t.Fatal("caller event did not settle")
		}
		if targetManager.Snapshot().State != pluginruntime.StateRunning {
			t.Fatalf("provider state = %s", targetManager.Snapshot().State)
		}
	})
}
