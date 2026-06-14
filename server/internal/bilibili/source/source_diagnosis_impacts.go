package source

import "github.com/RayleaBot/RayleaBot/server/internal/thirdparty"

func cooldownImpacts(cooldowns []requestCooldown, status Status) []string {
	impacts := make([]string, 0, 4)
	hasLive := false
	hasDynamic := false
	hasAutoFollow := false
	for _, cooldown := range cooldowns {
		switch normalizeCooldownScope(cooldown.Scope) {
		case bilibiliRequestCooldownLive:
			hasLive = true
		case bilibiliRequestCooldownDynamic:
			hasDynamic = true
		case bilibiliRequestCooldownAutoFollow:
			hasAutoFollow = true
		}
	}
	if hasLive {
		impacts = append(impacts, "直播状态暂时等待平台恢复。")
	} else {
		impacts = append(impacts, liveImpact(status))
	}
	if hasDynamic {
		impacts = append(impacts, "动态检查暂时等待平台恢复。")
	} else {
		impacts = append(impacts, dynamicImpact(status))
	}
	if hasAutoFollow {
		impacts = append(impacts, "自动关注暂时等待平台恢复。")
	}
	impacts = append(impacts, accountImpact(status.Accounts))
	return impacts
}

func liveImpact(status Status) string {
	if status.Live.WatchedRooms == 0 {
		return "当前没有直播监控目标。"
	}
	if status.Live.FailedRooms > 0 {
		return "直播状态仍会检查，但实时性可能降低。"
	}
	return "直播状态正常检查。"
}

func dynamicImpact(status Status) string {
	if !status.Dynamic.Enabled || status.Dynamic.WatchedUIDs == 0 {
		return "当前没有动态监控目标。"
	}
	if status.Dynamic.LastError != "" {
		return "动态检查当前存在错误。"
	}
	return "动态接收不受影响。"
}

func accountImpact(accounts []thirdparty.Account) string {
	for _, account := range accounts {
		if account.Credential.State == thirdparty.CredentialInvalid {
			return "CK 需要重新登录。"
		}
	}
	return "CK 有效，无需重新登录。"
}
