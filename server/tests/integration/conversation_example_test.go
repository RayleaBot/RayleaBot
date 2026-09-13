package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/sdk/go/pluginbuild"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/conversation"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

type conversationSender struct {
	messages chan string
	fail     atomic.Bool
}

func (s *conversationSender) send(segments []chatevent.MessageSegment) (chatevent.SendMessageResult, error) {
	if s.fail.Load() {
		return chatevent.SendMessageResult{}, errors.New("fixture prompt failure")
	}
	var text strings.Builder
	for _, segment := range segments {
		if segment.Type == "text" {
			value, _ := segment.Data["text"].(string)
			text.WriteString(value)
		}
	}
	s.messages <- text.String()
	return chatevent.SendMessageResult{MessageID: "fixture-outbound"}, nil
}
func (s *conversationSender) SendMessage(_ context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	return s.send(message.Segments)
}
func (s *conversationSender) SendReply(_ context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	return s.send(message.Segments)
}

func TestConversationExampleWithNativeSDKProcess(t *testing.T) {
	output := t.TempDir()
	artifact, err := pluginbuild.Build(t.Context(), pluginbuild.Config{PluginDir: testutil.RepoPath(t, "examples", "plugins", "example-conversation"), OutputDir: output, TargetPlatform: pluginbuild.CurrentPlatform(), KeepExpandedArtifact: true, SkipUIBuild: true})
	if err != nil {
		t.Fatal(err)
	}
	validator, err := config.Compile(testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	snapshots, _, err := catalog.Discover(catalog.DiscoverOptions{Validator: validator, Roots: []catalog.ScanRoot{{Label: "fixture", Path: filepath.Dir(artifact.ArtifactDir)}}, RepoRoot: output, Logger: logger})
	if err != nil || len(snapshots) != 1 || !snapshots[0].Valid {
		t.Fatalf("discover=%#v %v", snapshots, err)
	}
	for _, scenario := range []string{"three-rounds", "prompt-failure", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			cfg, _, err := config.Init(filepath.Join(root, "config", "user.yaml"), "")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Command = &config.CommandConfig{Prefixes: []string{"/"}}
			store, err := storage.Open(filepath.Join(root, "state.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			kv, err := pluginstore.NewKVSQLiteRepository(store)
			if err != nil {
				t.Fatal(err)
			}
			sender := &conversationSender{messages: make(chan string, 16)}
			sender.fail.Store(scenario == "prompt-failure")
			replies := outbound.NewReplyTargetCache(128)
			d := dispatch.New(logger, sender, replies, 16)
			r := conversation.New(conversation.Options{NotifyExpired: func(owner conversation.Owner, event chatevent.Event) {
				d.DispatchToProcess(context.Background(), owner.PluginID, owner.Done, event)
			}})
			t.Cleanup(r.Close)
			t.Cleanup(d.Close)
			snapshot := snapshots[0]
			snapshot.DesiredState = plugins.DesiredStateEnabled
			snapshot.RuntimeState = "running"
			view := catalog.New([]plugins.Snapshot{snapshot})
			permissions := plugins.NewPermissionView(plugins.PermissionViewDeps{Plugins: view})
			d.SetPermissionChecker(permissions.PermissionDeclared)
			local := actions.New(actions.Deps{CurrentConfig: func() config.Config { return cfg }, Logger: logger, PluginKV: kv, Conversations: r, MessageSender: actions.OutboundMessageSender(d), Permissions: permissions})
			manager := pluginruntime.NewManager(logger, pluginruntime.Options{
				ExecuteLocalAction: func(ctx context.Context, id, requestID string, action plugins.Action, event chatevent.Event) (map[string]any, error) {
					if scenario == "timeout" && action.Kind == "session.wait" {
						action.SessionTimeoutSeconds = 1
					}
					return local.Execute(ctx, id, requestID, action, event)
				},
				Events: pluginruntime.EventHooks{Before: func(id string, done <-chan struct{}, event chatevent.Event) bool {
					if event.Session == nil || event.EventType == "session.expired" {
						return true
					}
					return r.BeginInput(conversation.Owner{PluginID: id, Done: done}, event)
				}, Completed: func(id string, done <-chan struct{}, requestID string, event chatevent.Event, success bool) {
					r.CompleteParent(conversation.Owner{PluginID: id, Done: done}, requestID, event, success)
				}},
			})
			spec, err := pluginruntime.BuildSpec(snapshots[0], output, cfg.Runtime)
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.Start(t.Context(), spec, pluginruntime.InitPayload{Timezone: "UTC", Bots: []chatevent.BotIdentity{{SourceProtocol: "onebot11", SourceAdapter: "fixture", ID: "bot"}}, Config: map[string]any{}, Permissions: []string{"message.send"}, CommandPrefixes: []string{"/"}}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = manager.Stop(ctx)
			})
			d.Register("example-conversation", manager, snapshots[0].Events, snapshots[0].Commands, 1)
			b := bridge.New(logger, d)
			ingress := chatpolicy.NewIngress(chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return cfg }, Logger: logger, Plugins: view, Bridge: b, ReplyTargets: replies, Conversations: r})
			sequence := 0
			message := func(text string) chatevent.NormalizedEvent {
				sequence++
				return chatevent.NormalizedEvent{Kind: chatevent.EventKindMessageText, EventID: fmt.Sprint(sequence), MessageID: fmt.Sprint(sequence), BotID: "bot", SourceProtocol: "onebot11", SourceAdapter: "fixture", EventType: "message.group", Timestamp: time.Now().Unix(), ConversationType: "group", ConversationID: "group", SenderID: "actor", PlainText: text}
			}
			initial := message("/会话演示")
			ingress.HandleAdapterEvent(t.Context(), initial)
			countRows := func() int {
				t.Helper()
				var count int
				if err := store.Read.QueryRow("SELECT count(*) FROM plugin_kv WHERE plugin_id='example-conversation'").Scan(&count); err != nil {
					t.Fatal(err)
				}
				return count
			}
			readMessage := func() string {
				t.Helper()
				select {
				case value := <-sender.messages:
					return value
				case <-time.After(3 * time.Second):
					t.Fatal("native example did not send a message")
					return ""
				}
			}
			if scenario == "prompt-failure" {
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				defer cancel()
				if err := d.DrainPlugin("example-conversation").Wait(ctx); err != nil {
					t.Fatal(err)
				}
				if r.HasWaiting(chatevent.FromAdapter(initial)) || countRows() != 0 {
					t.Fatal("failed prompt retained conversation or wrote state")
				}
				return
			}
			if !strings.Contains(readMessage(), "角色") {
				t.Fatal("missing first question")
			}
			if scenario == "timeout" {
				if !strings.Contains(readMessage(), "超时") || countRows() != 0 {
					t.Fatal("timeout wrote a result or did not notify")
				}
				return
			}
			waitReady := func() {
				t.Helper()
				deadline := time.Now().Add(2 * time.Second)
				for !r.HasWaiting(chatevent.FromAdapter(initial)) {
					if time.Now().After(deadline) {
						t.Fatal("parent did not commit next wait")
					}
					time.Sleep(time.Millisecond)
				}
			}
			for _, input := range []string{"无效角色", "读者", "查看"} {
				waitReady()
				ingress.HandleAdapterEvent(t.Context(), message(input))
				readMessage()
				if countRows() != 0 {
					t.Fatal("example persisted before confirmation")
				}
			}
			waitReady()
			ingress.HandleAdapterEvent(t.Context(), message("确认"))
			if !strings.Contains(readMessage(), "已确认：读者 / 查看") || countRows() != 1 {
				t.Fatal("three-round result was not written exactly once")
			}
			var expiry int64
			if err := store.Read.QueryRow("SELECT expires_at_ms FROM plugin_kv WHERE plugin_id='example-conversation'").Scan(&expiry); err != nil || expiry <= time.Now().Add(4*time.Minute).UnixMilli() {
				t.Fatal("example result TTL missing")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			if err := d.DrainPlugin("example-conversation").Wait(ctx); err != nil {
				t.Fatal(err)
			}
			if r.HasWaiting(chatevent.FromAdapter(initial)) {
				t.Fatal("confirmed example retained conversation")
			}
		})
	}
}
