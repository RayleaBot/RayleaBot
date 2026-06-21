// Package thirdpartylogin is a compatibility shim that delegates to the new
// per-platform integration packages. New code should import the per-platform
// packages directly (weibo, douyin, netease_music, common).
package thirdpartylogin

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/douyin"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/netease_music"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/weibo"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

// Re-export common types with original names for backward compatibility.
type (
	CreateResult = common.CreateResult
	PollResult   = common.PollResult
	Options      = common.Options

	AccountValidator = common.AccountValidator
)

// Re-export error sentinels.
var (
	ErrUnsupportedPlatform  = common.ErrUnsupportedPlatform
	ErrLoginSessionNotFound = common.ErrLoginSessionNotFound
)

// NewService creates a QR login Service with the original Options signature.
func NewService(transport http.RoundTripper, now func() time.Time) *common.Service {
	return NewServiceWithOptions(Options{
		Transport: transport,
		Now:       now,
	})
}

// NewServiceWithOptions creates a QR login Service with per-platform providers.
func NewServiceWithOptions(options Options) *common.Service {
	httpClient := common.NewHTTPClient(options.Transport)
	douyinBrowser := douyin.NewChromedpBrowser(options.BrowserPath, options.BrowserArgs, options.Transport)
	providers := map[string]common.Provider{
		thirdparty.PlatformWeibo:        weibo.NewProvider(httpClient),
		thirdparty.PlatformDouyin:       douyin.NewProvider(httpClient, douyinBrowser),
		thirdparty.PlatformNeteaseMusic: netease_music.NewProvider(httpClient),
	}
	return common.NewService(providers, options.Now)
}

// NewAccountValidator creates a validator with per-platform check functions registered.
func NewAccountValidator(transport http.RoundTripper, now func() time.Time) *common.AccountValidator {
	v := common.NewAccountValidator(transport, now)
	v.RegisterPlatform(thirdparty.PlatformWeibo, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return weibo.FetchAccountProfile(ctx, client, cookies)
	})
	v.RegisterPlatform(thirdparty.PlatformDouyin, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return douyin.FetchAccountProfile(ctx, client, cookies)
	})
	v.RegisterPlatform(thirdparty.PlatformNeteaseMusic, func(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
		return netease_music.FetchAccountProfile(ctx, client, cookies)
	})
	return v
}
