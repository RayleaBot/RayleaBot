package localaction

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (s *Service) renderImageData(ctx context.Context, templateID string, data map[string]any, parentEvent runtime.Event) map[string]any {
	if !s.templateAcceptsRenderIdentity(ctx, templateID) {
		return data
	}

	merged := cloneRenderData(data)
	identity := s.renderIdentity(parentEvent)
	merged["user"] = identity.user
	merged["permission"] = identity.permission
	if identity.group != nil {
		merged["group"] = identity.group
	} else {
		delete(merged, "group")
	}
	return merged
}

func (s *Service) templateAcceptsRenderIdentity(ctx context.Context, templateID string) bool {
	if s == nil || s.renderer == nil {
		return false
	}

	_, source, err := s.renderer.GetTemplateSource(ctx, templateID)
	if err != nil {
		return false
	}

	properties, ok := source.InputSchemaJSON["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, hasUser := properties["user"]
	_, hasPermission := properties["permission"]
	return hasUser && hasPermission
}

type renderIdentity struct {
	user       map[string]any
	group      map[string]any
	permission map[string]any
}

func (s *Service) renderIdentity(event runtime.Event) renderIdentity {
	actor := event.Actor
	target := event.Target
	onebot := objectValue(event.PayloadFields["onebot"])
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
		user["avatar_url"] = "https://q1.qlogo.cn/g?b=qq&nk=" + url.QueryEscape(userID) + "&s=100"
	}
	if title := firstText(sender["title"]); title != "" {
		user["title"] = title
	}

	level := "member"
	if userID != "" && s.userIsSuperAdmin(userID) {
		level = "super_admin"
	} else {
		level = normalizePermissionLevel(firstText(actorRole, sender["role"]))
	}

	identity := renderIdentity{
		user: user,
		permission: map[string]any{
			"level": level,
		},
	}
	if isGroup {
		identity.group = map[string]any{}
		if groupName := firstText(targetName); groupName != "" {
			identity.group["name"] = groupName
		}
	}
	return identity
}

func (s *Service) userIsSuperAdmin(userID string) bool {
	for _, candidate := range s.config().Admin.SuperAdmins {
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

func cloneRenderData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data)+3)
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
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
