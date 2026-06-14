package localaction

import (
	"context"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type GrantView interface {
	CapabilityGranted(context.Context, string, string) bool
	StorageRootGranted(context.Context, string, string) bool
	GrantedHTTPHosts(context.Context, string) []string
	GrantedWebhookScope(context.Context, string, string) (plugins.WebhookScope, bool)
	ListPluginSnapshots() []plugins.Snapshot
}

type WebhookGateway interface {
	Expose(context.Context, string, runtimeaction.Action) (map[string]any, error)
}

type GovernanceService interface {
	ReadBlacklist(context.Context) (governance.BlacklistSnapshot, error)
	UpsertBlacklistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteBlacklistEntry(context.Context, string, string) error
	ReadWhitelist(context.Context) (governance.WhitelistSnapshot, error)
	SetWhitelistEnabled(context.Context, bool) (governance.WhitelistStateResponse, error)
	UpsertWhitelistEntry(context.Context, string, string, string) (governance.EntryResponse, error)
	DeleteWhitelistEntry(context.Context, string, string) error
	ReadCommandPolicy(context.Context) (governance.CommandPolicyResponse, error)
}

type ThirdPartyAccounts interface {
	ListEnabled(context.Context, string) ([]thirdparty.Account, error)
	ReadCookie(context.Context, thirdparty.Account) (string, error)
	UpdateCookie(context.Context, thirdparty.Account, string) error
	MarkUsed(context.Context, thirdparty.Account) error
}

type BilibiliSession interface {
	PrepareCookie(context.Context, string) (bilibilisession.PreparedCookie, error)
	SignURL(context.Context, string, string) (string, error)
	InvalidateWBI()
}
