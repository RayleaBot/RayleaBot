package defaultmodules

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

var thirdPartyAccountIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,62}[a-z0-9])?$`)

func init() {
	register(Metadata{
		Action:         "thirdparty.account.read",
		Capability:     "thirdparty.account.read",
		RequestSchema:  "plugin-protocol.action_thirdparty_account_read",
		ResponseSchema: "plugin-protocol.local_action_result",
		ReadsSecret:    true,
		AuditFields:    []string{"plugin_id", "platform", "account_id", "count"},
		ErrorCodes:     commonErrorCodes("platform.invalid_request"),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return executeThirdPartyAccountRead(ctx, deps, req)
		}
	})
}

func executeThirdPartyAccountRead(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "thirdparty.account.read") {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: "thirdparty.account.read capability is not declared"}
	}

	platform, err := thirdparty.NormalizePlatform(req.Action.ThirdPartyAccountPlatform)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "platform.invalid_request", Message: "thirdparty.account.read platform is invalid"}
	}
	if !thirdPartyAccountPlatformAllowed(deps.Capabilities.ThirdPartyAccountPlatforms(ctx, req.PluginID), platform) {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: "thirdparty.account.read platform is outside declared capability parameters"}
	}
	accountID := strings.TrimSpace(req.Action.ThirdPartyAccountID)
	if accountID != "" && !thirdPartyAccountIDPattern.MatchString(accountID) {
		return nil, &runtimemanager.Error{Code: "platform.invalid_request", Message: "thirdparty.account.read account_id is invalid"}
	}
	if deps.ThirdParty == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "thirdparty.account.read store is not available"}
	}

	accounts, err := deps.ThirdParty.ListEnabled(ctx, platform)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "thirdparty.account.read failed", Err: err}
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
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "thirdparty.account.read failed", Err: err}
		}
		items = append(items, thirdPartyAccountReadItem(account, cookie))
	}

	return map[string]any{
		"platform": platform,
		"accounts": items,
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
