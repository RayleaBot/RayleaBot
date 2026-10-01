package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type drainingReply struct {
	shell   *onebot11.Shell
	started chan struct{}
	release chan struct{}
	result  chan error
}

func (*drainingReply) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}
func (*drainingReply) ReadyForEvents() bool { return true }
func (d *drainingReply) DeliverEvent(ctx context.Context, _ chatevent.Event) (plugins.Delivery, error) {
	close(d.started)
	select {
	case <-d.release:
	case <-ctx.Done():
		d.result <- ctx.Err()
		return plugins.Delivery{}, ctx.Err()
	}
	reply, err := d.shell.SendReply(ctx, chatevent.OutboundMessageReply{TargetType: "group", TargetID: "2001", ReplyToMessageID: "123", Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "accepted reply"}}}})
	if err == nil && reply.MessageID != "456" {
		err = fmt.Errorf("receipt = %#v", reply)
	}
	d.result <- err
	return plugins.Delivery{}, err
}

func TestAppDrainKeepsOutboundTransportAndReceiptReaderAlive(t *testing.T) {
	peerDone := make(chan struct{})
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow(); close(peerDone) }()
		_ = wsjson.Write(r.Context(), conn, map[string]any{"post_type": "meta_event", "meta_event_type": "lifecycle", "sub_type": "enable"})
		for {
			var request map[string]any
			if err := wsjson.Read(r.Context(), conn, &request); err != nil {
				return
			}
			if err := wsjson.Write(r.Context(), conn, map[string]any{"status": "ok", "retcode": 0, "echo": request["echo"], "data": map[string]any{"message_id": 456}}); err != nil {
				return
			}
		}
	}))
	t.Cleanup(peer.Close)
	shell := onebot11.NewForTest("fixture", config.OneBotConfig{ForwardWS: config.OneBotTransportConfig{Enabled: true, URL: "ws" + strings.TrimPrefix(peer.URL, "http")}}, config.AdapterConfig{ConnectTimeoutSeconds: 1}, nil, true)
	protocol, err := adapterservice.NewService(&appRuntimeState{}, adapterservice.Instances{OneBot11: map[string]*onebot11.Shell{"fixture": shell}})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := dispatch.New(nil, nil, nil, 4)
	ingress := chatpolicy.NewIngress(chatpolicy.IngressDeps{})
	app := &App{services: Services{Protocol: protocol, EventIngress: ingress}, eventStack: EventState{Dispatcher: dispatcher}}
	t.Cleanup(func() { _ = app.Close() })
	supervisor := newRunSupervisor(t.Context())
	started := make(chan struct{})
	close(started)
	if !app.setRunSupervisor(supervisor, started) {
		t.Fatal("supervisor rejected")
	}
	if err := app.startAdapters(supervisor.Context()); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	for shell.Snapshot().State != onebot11.StateConnected {
		select {
		case <-deadline:
			t.Fatal("adapter did not connect")
		case <-time.After(time.Millisecond):
		}
	}
	delivery := &drainingReply{shell: shell, started: make(chan struct{}), release: make(chan struct{}), result: make(chan error, 1)}
	dispatcher.Register("fixture", delivery, nil, nil, 1)
	dispatcher.DispatchToPlugin(t.Context(), "fixture", chatevent.Event{EventID: "accepted"})
	<-delivery.started
	app.requestShutdown(systemsvc.StopIntentStop)
	if supervisor.Context().Err() == nil {
		t.Fatal("shutdown did not cancel background work")
	}
	closed := make(chan error, 1)
	go func() { closed <- app.Close() }()
	deadline = time.After(3 * time.Second)
	for !dispatcher.IsClosed() {
		select {
		case <-deadline:
			t.Fatal("drain did not begin")
		case <-time.After(time.Millisecond):
		}
	}
	if result := dispatcher.DispatchToPlugin(t.Context(), "fixture", chatevent.Event{EventID: "late"}); result.Outcome == dispatch.OutcomeDelivered {
		t.Fatal("drain admitted a late event")
	}
	close(delivery.release)
	if err := <-delivery.result; err != nil {
		t.Fatalf("accepted reply failed during drain: %v", err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if shell.Snapshot().State != onebot11.StateStopped {
		t.Fatal("adapter was not stopped after drain")
	}
	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("transport remained open")
	}
}
