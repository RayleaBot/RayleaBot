package localaction

import (
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func RenderIdentityData(cfg config.Config, event runtime.Event) RenderIdentity {
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
	if userID != "" && renderIdentityUserIsSuperAdmin(cfg, userID) {
		level = "super_admin"
	} else {
		level = normalizePermissionLevel(firstText(actorRole, sender["role"]))
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

func renderIdentityUserIsSuperAdmin(cfg config.Config, userID string) bool {
	for _, candidate := range cfg.Admin.SuperAdmins {
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
