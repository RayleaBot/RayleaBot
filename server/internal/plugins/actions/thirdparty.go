package actions

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

var thirdPartyAccountIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,62}[a-z0-9])?$`)

func thirdPartyAccountReadRegistrar() registrar {
	return registrar{
		kind: "thirdparty.account.read",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeThirdPartyAccountRead(ctx, deps, req)
			}
		},
	}
}

func thirdPartyAccountValidateRegistrar() registrar {
	return registrar{
		kind: "thirdparty.account.validate",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeThirdPartyAccountValidate(ctx, deps, req)
			}
		},
	}
}

func executeThirdPartyAccountRead(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "thirdparty.account.read") {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "thirdparty.account.read permission is not declared"}
	}

	platform, err := thirdparty.NormalizePlatform(req.Action.ThirdPartyAccountPlatform)
	if err != nil {
		return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "thirdparty.account.read platform is invalid"}
	}
	if !thirdPartyAccountPlatformAllowed(deps.Permissions.PermissionPlatforms(ctx, req.PluginID, "thirdparty.account.read"), platform) {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "thirdparty.account.read platform is outside declared permission parameters"}
	}
	accountID := strings.TrimSpace(req.Action.ThirdPartyAccountID)
	if accountID != "" && !thirdPartyAccountIDPattern.MatchString(accountID) {
		return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "thirdparty.account.read account_id is invalid"}
	}
	if deps.ThirdParty == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "thirdparty.account.read store is not available"}
	}

	accounts, err := deps.ThirdParty.ListEnabled(ctx, platform)
	if err != nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "thirdparty.account.read failed", Err: err}
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
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "thirdparty.account.read failed", Err: err}
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
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "thirdparty.account.validate permission is not declared"}
	}

	platform, err := thirdparty.NormalizePlatform(req.Action.ThirdPartyAccountPlatform)
	if err != nil {
		return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "thirdparty.account.validate platform is invalid"}
	}
	if !thirdPartyAccountPlatformAllowed(deps.Permissions.PermissionPlatforms(ctx, req.PluginID, "thirdparty.account.validate"), platform) {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "thirdparty.account.validate platform is outside declared permission parameters"}
	}
	accountID := strings.TrimSpace(req.Action.ThirdPartyAccountID)
	if !thirdPartyAccountIDPattern.MatchString(accountID) {
		return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "thirdparty.account.validate account_id is invalid"}
	}
	observation := strings.TrimSpace(req.Action.ThirdPartyAccountObservation)
	if observation != "auth_rejected" && observation != "session_blocked" {
		return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "thirdparty.account.validate observation is invalid"}
	}
	httpStatus := req.Action.ThirdPartyAccountHTTPStatus
	if httpStatus != 0 && (httpStatus < 100 || httpStatus > 599) {
		return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "thirdparty.account.validate http_status is invalid"}
	}
	if deps.AccountValidation == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "thirdparty.account.validate service is not available"}
	}

	accepted, reason, err := deps.AccountValidation.RequestPluginValidation(ctx, req.PluginID, platform, accountID, observation, httpStatus)
	if err != nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "thirdparty.account.validate failed", Err: err}
	}
	return map[string]any{"accepted": accepted, "reason": reason}, nil
}

func thirdPartyResolveRegistrar() registrar {
	return registrar{
		kind: "thirdparty.resolve",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeThirdPartyResolve(ctx, deps, req)
			}
		},
	}
}

func executeThirdPartyResolve(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "thirdparty.resolve") {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "thirdparty.resolve permission is not declared"}
	}

	platform, err := thirdparty.NormalizePlatform(req.Action.ThirdPartyAccountPlatform)
	if err != nil || platform != thirdparty.PlatformDouyin {
		return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "thirdparty.resolve platform is invalid"}
	}
	if !thirdPartyAccountPlatformAllowed(deps.Permissions.PermissionPlatforms(ctx, req.PluginID, "thirdparty.resolve"), platform) {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "thirdparty.resolve platform is outside declared permission parameters"}
	}
	query := strings.TrimSpace(req.Action.ThirdPartyResolveQuery)
	// schema maxLength 按 Unicode 码点计，这里用 rune 计数保持一致，
	// 避免多字节昵称（如 emoji）被字节长度误拒。
	if query == "" || utf8.RuneCountInString(query) > 64 {
		return nil, &plugins.Error{Code: errorcodes.PlatformInvalidRequest, Message: "thirdparty.resolve query is invalid"}
	}
	if deps.ThirdPartyResolve == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "thirdparty.resolve service is not available"}
	}

	cookieSets := make([]map[string]string, 0)
	// store CK 先注入作为基线：store 查询已排除 invalid 凭据，质量可控；
	// 插件显式 CK（当前会话更新鲜）后注入，同名字段覆盖基线。
	if deps.ThirdParty != nil {
		accounts, err := deps.ThirdParty.ListEnabled(ctx, platform)
		if err != nil {
			return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "thirdparty.resolve account read failed", Err: err}
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
			deps.Logger.Warn("插件查找三方用户失败", "component", "plugin_action", "plugin_id", req.PluginID, "platform", platform, "query", query, "err", err.Error())
		}
		// 登录 profile 槽忙映射为 409，可重试；其余上游失败映射为 502。
		if errors.Is(err, thirdparty.ErrQRLoginBrowserBusy) {
			return nil, &plugins.Error{Code: errorcodes.PlatformResourceBusy, Message: "thirdparty.resolve browser profile is busy", Err: err}
		}
		return nil, &plugins.Error{Code: errorcodes.PlatformUpstreamRequestFailed, Message: "thirdparty.resolve failed", Err: err}
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
