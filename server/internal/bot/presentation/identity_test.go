package presentation

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

func TestIdentityKeepsProtocolIdentityAndPermissionsSeparate(t *testing.T) {
	event := chatevent.Event{
		SourceProtocol: "onebot11",
		Actor:          &chatevent.Actor{ID: "10001", Nickname: "actor"},
		Target:         &chatevent.Target{Type: "group", Name: "group"},
		PayloadFields:  map[string]any{"onebot": map[string]any{"sender": map[string]any{"user_id": "10001", "card": "group card", "title": "title"}}},
	}
	onebot := IdentityData([]string{"10001"}, event)
	if onebot.User["nickname"] != "group card" || onebot.User["avatar_url"] == nil || onebot.Permission["level"] != "super_admin" || onebot.Group["name"] != "group" {
		t.Fatalf("OneBot identity projection: %#v", onebot)
	}
	event.SourceProtocol = "qqofficial"
	official := IdentityData([]string{"10001"}, event)
	if official.User["nickname"] != "actor" || official.User["avatar_url"] != nil || official.User["title"] != nil || official.Permission["level"] != "member" {
		t.Fatalf("QQ identity inherited OneBot fields or admin permission: %#v", official)
	}
	event.Target.Type = "private"
	if identity := IdentityData(nil, event); identity.Group != nil {
		t.Fatal("private render retained a group")
	}
}
