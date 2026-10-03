package chatpolicy

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"testing"
)

type protocolPolicyCatalog []plugins.Snapshot

func (c protocolPolicyCatalog) List() []plugins.Snapshot         { return c }
func (c protocolPolicyCatalog) Commands() []plugins.CommandEntry { return plugins.CommandEntries(c) }

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
