package render

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
)

type RenderIdentity struct {
	User       map[string]any
	Group      map[string]any
	Permission map[string]any
}

func RenderIdentityData(superAdmins []string, event chatevent.Event) RenderIdentity {
	actor := event.Actor
	target := event.Target
	onebot := map[string]any{}
	if event.SourceProtocol == "onebot11" {
		onebot = objectValue(event.PayloadFields["onebot"])
	}
	sender := objectValue(onebot["sender"])

	var actorID, actorNickname, actorRole string
	if actor != nil {
		actorID = actor.ID
		actorNickname = actor.Nickname
		actorRole = actor.Role
	}

	targetType := ""
	targetName := ""
	if target != nil {
		targetType = target.Type
		targetName = target.Name
	}
	isGroup := strings.EqualFold(strings.TrimSpace(targetType), "group")

	userID := firstText(sender["user_id"], actorID, onebot["user_id"])
	nickname := ""
	if isGroup {
		nickname = firstText(sender["card"], actorNickname, sender["nickname"], userID)
	} else {
		nickname = firstText(actorNickname, sender["nickname"], userID)
	}

	user := map[string]any{}
	if nickname != "" {
		user["nickname"] = nickname
	}
	if userID != "" {
		user["id"] = userID
		if event.SourceProtocol == "onebot11" {
			user["avatar_url"] = "https://q1.qlogo.cn/g?b=qq&nk=" + url.QueryEscape(userID) + "&s=100"
		}
	}
	if title := firstText(sender["title"]); title != "" {
		user["title"] = title
	}

	level := normalizePermissionLevel(firstText(actorRole, sender["role"]))
	if event.SourceProtocol == "onebot11" && userID != "" && renderIdentityUserIsSuperAdmin(superAdmins, userID) {
		level = "super_admin"
	}

	identity := RenderIdentity{
		User: user,
		Permission: map[string]any{
			"level": level,
		},
	}
	if isGroup {
		identity.Group = map[string]any{}
		if groupName := firstText(targetName); groupName != "" {
			identity.Group["name"] = groupName
		}
	}
	return identity
}

func CloneRenderData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data)+3)
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func renderIdentityUserIsSuperAdmin(superAdmins []string, userID string) bool {
	for _, candidate := range superAdmins {
		if strings.TrimSpace(candidate) == userID {
			return true
		}
	}
	return false
}

func normalizePermissionLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "owner":
		return "owner"
	case "admin", "administrator", "group_admin":
		return "admin"
	default:
		return "member"
	}
}

func objectValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func firstText(values ...any) string {
	for _, value := range values {
		text := textValue(value)
		if text != "" {
			return text
		}
	}
	return ""
}

func textValue(value any) string {
	if value == nil {
		return ""
	}
	if _, ok := value.(bool); ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
