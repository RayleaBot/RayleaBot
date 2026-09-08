package actions

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

var thirdPartyAccountIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,62}[a-z0-9])?$`)

func thirdPartyAccountReadRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:         "thirdparty.account.read",
			Permission:     "thirdparty.account.read",
			RequestSchema:  "plugin-protocol.action_thirdparty_account_read",
			ResponseSchema: "plugin-protocol.local_action_result",
			ReadsSecret:    true,
			AuditFields:    []string{"plugin_id", "platform", "account_id", "count"},
			ErrorCodes:     commonErrorCodes("platform.invalid_request"),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeThirdPartyAccountRead(ctx, deps, req)
			}
		},
	}
}

func thirdPartyAccountValidateRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:         "thirdparty.account.validate",
			Permission:     "thirdparty.account.validate",
			RequestSchema:  "plugin-protocol.action_thirdparty_account_validate",
			ResponseSchema: "plugin-protocol.local_action_result",
			AuditFields:    []string{"plugin_id", "platform", "account_id", "observation", "http_status", "accepted", "reason"},
			ErrorCodes:     commonErrorCodes("platform.invalid_request"),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeThirdPartyAccountValidate(ctx, deps, req)
			}
		},
	}
}

func executeThirdPartyAccountRead(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "thirdparty.account.read") {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "thirdparty.account.read permission is not declared"}
	}

	platform, err := thirdparty.NormalizePlatform(req.Action.ThirdPartyAccountPlatform)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "thirdparty.account.read platform is invalid"}
	}
	if !thirdPartyAccountPlatformAllowed(deps.Permissions.PermissionPlatforms(ctx, req.PluginID, "thirdparty.account.read"), platform) {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "thirdparty.account.read platform is outside declared permission parameters"}
	}
	accountID := strings.TrimSpace(req.Action.ThirdPartyAccountID)
	if accountID != "" && !thirdPartyAccountIDPattern.MatchString(accountID) {
		return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "thirdparty.account.read account_id is invalid"}
	}
	if deps.ThirdParty == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "thirdparty.account.read store is not available"}
	}

	accounts, err := deps.ThirdParty.ListEnabled(ctx, platform)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "thirdparty.account.read failed", Err: err}
	}

	items := make([]map[string]any, 0, len(accounts))
	for _, account := range accounts {
		if accountID != "" && account.AccountID != accountID {
			continue
		}
		if !account.Configured {
			continue
		}
		cookie, err := deps.ThirdParty.ReadCookie(ctx, account)
		if errors.Is(err, secrets.ErrNotFound) || strings.TrimSpace(cookie) == "" {
			continue
		}
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "thirdparty.account.read failed", Err: err}
		}
		items = append(items, thirdPartyAccountReadItem(account, cookie))
	}

	return map[string]any{
		"platform": platform,
		"accounts": items,
	}, nil
}

func executeThirdPartyAccountValidate(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "thirdparty.account.validate") {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "thirdparty.account.validate permission is not declared"}
	}

	platform, err := thirdparty.NormalizePlatform(req.Action.ThirdPartyAccountPlatform)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "thirdparty.account.validate platform is invalid"}
	}
	if !thirdPartyAccountPlatformAllowed(deps.Permissions.PermissionPlatforms(ctx, req.PluginID, "thirdparty.account.validate"), platform) {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "thirdparty.account.validate platform is outside declared permission parameters"}
	}
	accountID := strings.TrimSpace(req.Action.ThirdPartyAccountID)
	if !thirdPartyAccountIDPattern.MatchString(accountID) {
		return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "thirdparty.account.validate account_id is invalid"}
	}
	observation := strings.TrimSpace(req.Action.ThirdPartyAccountObservation)
	if observation != "auth_rejected" && observation != "session_blocked" {
		return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "thirdparty.account.validate observation is invalid"}
	}
	httpStatus := req.Action.ThirdPartyAccountHTTPStatus
	if httpStatus != 0 && (httpStatus < 100 || httpStatus > 599) {
		return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "thirdparty.account.validate http_status is invalid"}
	}
	if deps.AccountValidation == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "thirdparty.account.validate service is not available"}
	}

	accepted, reason, err := deps.AccountValidation.RequestPluginValidation(ctx, req.PluginID, platform, accountID, observation, httpStatus)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "thirdparty.account.validate failed", Err: err}
	}
	return map[string]any{"accepted": accepted, "reason": reason}, nil
}

func thirdPartyResolveRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:         "thirdparty.resolve",
			Permission:     "thirdparty.resolve",
			RequestSchema:  "plugin-protocol.action_thirdparty_resolve",
			ResponseSchema: "plugin-protocol.local_action_result",
			ReadsSecret:    true,
			AuditFields:    []string{"plugin_id", "platform", "query", "count"},
			ErrorCodes:     commonErrorCodes("platform.invalid_request", "platform.resource_busy", "platform.upstream_request_failed"),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeThirdPartyResolve(ctx, deps, req)
			}
		},
	}
}

func executeThirdPartyResolve(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "thirdparty.resolve") {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "thirdparty.resolve permission is not declared"}
	}

	platform, err := thirdparty.NormalizePlatform(req.Action.ThirdPartyAccountPlatform)
	if err != nil || platform != thirdparty.PlatformDouyin {
		return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "thirdparty.resolve platform is invalid"}
	}
	if !thirdPartyAccountPlatformAllowed(deps.Permissions.PermissionPlatforms(ctx, req.PluginID, "thirdparty.resolve"), platform) {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "thirdparty.resolve platform is outside declared permission parameters"}
	}
	query := strings.TrimSpace(req.Action.ThirdPartyResolveQuery)
	// schema maxLength 按 Unicode 码点计，这里用 rune 计数保持一致，
	// 避免多字节昵称（如 emoji）被字节长度误拒。
	if query == "" || utf8.RuneCountInString(query) > 64 {
		return nil, &pluginruntime.Error{Code: "platform.invalid_request", Message: "thirdparty.resolve query is invalid"}
	}
	if deps.ThirdPartyResolve == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "thirdparty.resolve service is not available"}
	}

	cookieSets := make([]map[string]string, 0)
	// store CK 先注入作为基线：store 查询已排除 invalid 凭据，质量可控；
	// 插件显式 CK（当前会话更新鲜）后注入，同名字段覆盖基线。
	if deps.ThirdParty != nil {
		accounts, err := deps.ThirdParty.ListEnabled(ctx, platform)
		if err != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "thirdparty.resolve account read failed", Err: err}
		}
		for _, account := range accounts {
			if !account.Configured {
				continue
			}
			cookie, err := deps.ThirdParty.ReadCookie(ctx, account)
			if err != nil || strings.TrimSpace(cookie) == "" {
				continue
			}
			cookieSets = append(cookieSets, thirdparty.CookieMapFromHeader(cookie))
		}
	}
	if cookie := strings.TrimSpace(req.Action.ThirdPartyResolveCookie); cookie != "" {
		cookieSets = append(cookieSets, thirdparty.CookieMapFromHeader(cookie))
	}

	profiles, exact, err := deps.ThirdPartyResolve.ResolveUser(ctx, query, cookieSets)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Warn(fmt.Sprintf("插件 %s 查找 %s 用户“%s”失败：%s", req.PluginID, platform, query, err.Error()), "component", "plugin_action", "plugin_id", req.PluginID, "platform", platform, "query", query, "err", err.Error())
		}
		// 登录 profile 槽忙映射为 409，可重试；其余上游失败映射为 502。
		if errors.Is(err, thirdparty.ErrQRLoginBrowserBusy) {
			return nil, &pluginruntime.Error{Code: "platform.resource_busy", Message: "thirdparty.resolve browser profile is busy", Err: err}
		}
		return nil, &pluginruntime.Error{Code: "platform.upstream_request_failed", Message: "thirdparty.resolve failed", Err: err}
	}
	items := make([]map[string]any, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, map[string]any{
			"uid":        profile.UID,
			"unique_id":  profile.UniqueID,
			"nickname":   profile.Nickname,
			"avatar_url": profile.AvatarURL,
		})
	}
	return map[string]any{
		"platform": platform,
		"profiles": items,
		"exact":    exact,
	}, nil
}

func thirdPartyAccountPlatformAllowed(allowed []string, platform string) bool {
	platform = strings.TrimSpace(platform)
	for _, value := range allowed {
		if strings.TrimSpace(value) == platform {
			return true
		}
	}
	return false
}

func thirdPartyAccountReadItem(account thirdparty.Account, cookie string) map[string]any {
	return map[string]any{
		"platform":   account.Platform,
		"account_id": account.AccountID,
		"label":      account.Label,
		"enabled":    account.Enabled,
		"configured": account.Configured,
		"profile": map[string]any{
			"uid":        account.Profile.UID,
			"nickname":   account.Profile.Nickname,
			"avatar_url": account.Profile.AvatarURL,
		},
		"credential_state": account.Credential.State,
		"cookie": map[string]any{
			"secret": true,
			"value":  cookie,
		},
	}
}
