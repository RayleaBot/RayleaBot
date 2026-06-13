package bilibili

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func invalidCredentialCause(accounts []thirdparty.Account) *DiagnosisCause {
	for _, account := range accounts {
		if account.Credential.State != thirdparty.CredentialInvalid {
			continue
		}
		detail := "账号 " + account.AccountID + " 的 CK 无效。"
		if strings.TrimSpace(account.Label) != "" {
			detail = account.Label + " 的 CK 无效。"
		}
		return &DiagnosisCause{
			Scope:     "account",
			Code:      "credential_invalid",
			Title:     "CK 无效",
			Detail:    detail,
			LastError: account.Credential.LastError,
		}
	}
	return nil
}

func cooldownCause(cooldown requestCooldown) DiagnosisCause {
	scope := normalizeCooldownScope(cooldown.Scope)
	title := "平台暂时限制请求"
	detail := "Bilibili 暂时限制部分请求，等待结束后会自动重试。"
	switch scope {
	case bilibiliRequestCooldownLive:
		title = "直播请求被平台限制"
		detail = "直播状态检查暂时等待平台恢复。"
	case bilibiliRequestCooldownDynamic:
		title = "动态请求被平台限制"
		detail = "动态检查暂时等待平台恢复。"
	case bilibiliRequestCooldownAutoFollow:
		title = "自动关注请求被平台限制"
		detail = "自动关注暂时等待平台恢复。"
	}
	return DiagnosisCause{
		Scope:     scope,
		Code:      cooldown.Code,
		Title:     title,
		Detail:    detail,
		LastError: cooldown.LastError,
		RetryAt:   timePtr(cooldown.Until),
	}
}
