package localaction

import (
	"context"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
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
	Expose(context.Context, string, runtime.Action) (map[string]any, error)
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
	PrepareCookie(context.Context, string) (source.PreparedCookie, error)
	SignURL(context.Context, string, string) (string, error)
	InvalidateWBI()
}
