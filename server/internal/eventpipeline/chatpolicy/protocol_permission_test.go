package chatpolicy

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"sync"
	"testing"
)

type protocolPolicyCatalog []plugins.Snapshot

func (c protocolPolicyCatalog) List() []plugins.Snapshot { return c }

func TestQQCommandEnforcesSamePermissionAsOneBot(t *testing.T) {
	s := New(Deps{CurrentConfig: func() config.Config { return config.Config{Command: &config.CommandConfig{Prefixes: []string{"/"}}} }, Plugins: protocolPolicyCatalog{{PluginID: "restricted", Valid: true, RegistrationState: "installed", DesiredState: "enabled", Commands: []plugins.Command{{Name: "danger", Permission: "super_admin"}}}}})
	event := chatevent.NormalizedEvent{Kind: chatevent.EventKindMessage, SourceProtocol: "onebot11", SourceAdapter: "onebot11", EventType: "message.group", SenderID: "ordinary-user", ActorRole: "member", ConversationType: "group", ConversationID: "group-fixture", PlainText: "/danger"}
	if _, allowed := s.Apply(context.Background(), event); allowed {
		t.Fatal("control OneBot command was not rejected")
	}
	event.Kind = chatevent.EventKind("qqofficial", chatevent.FamilyMessage)
	event.SourceProtocol = "qqofficial"
	event.SourceAdapter = "qq-official"
	if _, allowed := s.Apply(context.Background(), event); allowed {
		t.Fatal("same non-admin command bypassed permission because kind=qqofficial.message")
	}
}

func TestConcurrentAdaptersApplyChatPolicy(t *testing.T) {
	ingress := NewIngress(IngressDeps{})
	var workers sync.WaitGroup
	start := make(chan struct{})
	for _, protocol := range []string{"onebot11", "qqofficial"} {
		workers.Go(func() {
			<-start
			for range 64 {
				event := chatevent.NormalizedEvent{Kind: chatevent.EventKind(protocol, chatevent.FamilyMessage), SourceProtocol: protocol, SenderID: "fixture", ConversationType: "private", ConversationID: "fixture", PlainText: "hello"}
				if _, allowed := ingress.ApplyChatPolicy(context.Background(), event); !allowed {
					t.Error("ordinary message rejected")
				}
			}
		})
	}
	close(start)
	workers.Wait()
}
