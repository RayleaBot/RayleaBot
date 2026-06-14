package source

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func (s *Source) primaryAccountCookie(ctx context.Context) (thirdparty.Account, string, error) {
	return s.accountCookieFromOffset(ctx, 0)
}
func (s *Source) accountCookieForLive(ctx context.Context) (thirdparty.Account, string, error) {
	result, cookie, err := s.accountCookieFromOffset(ctx, s.liveAccountOffset)
	if err == nil {
		s.liveAccountOffset++
	}
	return result, cookie, err
}
func (s *Source) accountCookieForDynamic(ctx context.Context) (thirdparty.Account, string, error) {
	result, cookie, err := s.accountCookieFromOffset(ctx, s.dynamicAccountOffset)
	if err == nil {
		s.dynamicAccountOffset++
	}
	return result, cookie, err
}
func (s *Source) accountCookieFromOffset(ctx context.Context, offset int) (thirdparty.Account, string, error) {
	accounts, err := s.accounts.ListEnabled(ctx, thirdparty.PlatformBilibili)
	if err != nil {
		return thirdparty.Account{}, "", err
	}
	if len(accounts) == 0 {
		return thirdparty.Account{}, "", secrets.ErrNotFound
	}
	start := offset % len(accounts)
	for i := 0; i < len(accounts); i++ {
		account := accounts[(start+i)%len(accounts)]
		cookie, err := s.accounts.ReadCookie(ctx, account)
		if err == nil && strings.TrimSpace(cookie) != "" {
			if s.session != nil {
				prepared, prepareErr := s.session.PrepareCookie(ctx, cookie)
				if prepareErr != nil {
					if isBilibiliAuthError(prepareErr) {
						checkedAt := s.now()
						_ = s.accounts.UpdateCredentialStatus(ctx, account.Platform, account.AccountID, account.Profile, thirdparty.CredentialStatus{
							State:     thirdparty.CredentialInvalid,
							CheckedAt: &checkedAt,
							LastError: prepareErr.Error(),
						})
						continue
					}
					return thirdparty.Account{}, "", prepareErr
				}
				if prepared.Cookie != "" && prepared.Cookie != cookie && (prepared.Refreshed || prepared.Enriched) {
					if updateErr := s.accounts.UpdateCookie(ctx, account, prepared.Cookie); updateErr != nil {
						return thirdparty.Account{}, "", updateErr
					}
					cookie = prepared.Cookie
				}
			}
			_ = s.accounts.MarkUsed(ctx, account)
			return account, cookie, nil
		}
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return thirdparty.Account{}, "", err
		}
	}
	return thirdparty.Account{}, "", secrets.ErrNotFound
}
