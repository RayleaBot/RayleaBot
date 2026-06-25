package source

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type accountUsageManager struct {
	accounts *thirdparty.Service
	session  *bilibiliSession.SessionClient
	now      func() time.Time

	mu            sync.Mutex
	liveOffset    int
	dynamicOffset int
}

func newAccountUsageManager(accounts *thirdparty.Service, session *bilibiliSession.SessionClient, now func() time.Time) *accountUsageManager {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &accountUsageManager{accounts: accounts, session: session, now: now}
}

func (m *accountUsageManager) PrimaryCookie(ctx context.Context) (thirdparty.Account, string, error) {
	return m.cookieFromOffset(ctx, 0)
}

func (m *accountUsageManager) LiveCookie(ctx context.Context) (thirdparty.Account, string, error) {
	m.mu.Lock()
	offset := m.liveOffset
	m.mu.Unlock()
	account, cookie, err := m.cookieFromOffset(ctx, offset)
	if err == nil {
		m.mu.Lock()
		m.liveOffset++
		m.mu.Unlock()
	}
	return account, cookie, err
}

func (m *accountUsageManager) DynamicCookie(ctx context.Context) (thirdparty.Account, string, error) {
	m.mu.Lock()
	offset := m.dynamicOffset
	m.mu.Unlock()
	account, cookie, err := m.cookieFromOffset(ctx, offset)
	if err == nil {
		m.mu.Lock()
		m.dynamicOffset++
		m.mu.Unlock()
	}
	return account, cookie, err
}

func (m *accountUsageManager) IsCookieMissing(err error) bool {
	return errors.Is(err, secrets.ErrNotFound)
}

func (m *accountUsageManager) cookieFromOffset(ctx context.Context, offset int) (thirdparty.Account, string, error) {
	if m == nil || m.accounts == nil {
		return thirdparty.Account{}, "", secrets.ErrNotFound
	}
	accounts, err := m.accounts.ListEnabled(ctx, thirdparty.PlatformBilibili)
	if err != nil {
		return thirdparty.Account{}, "", err
	}
	if len(accounts) == 0 {
		return thirdparty.Account{}, "", secrets.ErrNotFound
	}
	start := offset % len(accounts)
	for i := 0; i < len(accounts); i++ {
		account := accounts[(start+i)%len(accounts)]
		cookie, err := m.accounts.ReadCookie(ctx, account)
		if err == nil && strings.TrimSpace(cookie) != "" {
			prepared, prepareErr := m.prepareCookie(ctx, cookie)
			if prepareErr != nil {
				if bilibiliSession.IsAuthError(prepareErr) {
					checkedAt := m.now()
					_ = m.accounts.UpdateCredentialStatus(ctx, account.Platform, account.AccountID, account.Profile, thirdparty.CredentialStatus{
						State:     thirdparty.CredentialInvalid,
						CheckedAt: &checkedAt,
						LastError: prepareErr.Error(),
					})
					continue
				}
				return thirdparty.Account{}, "", prepareErr
			}
			if prepared != cookie {
				if updateErr := m.accounts.UpdateCookie(ctx, account, prepared); updateErr != nil {
					return thirdparty.Account{}, "", updateErr
				}
				cookie = prepared
			}
			_ = m.accounts.MarkUsed(ctx, account)
			return account, cookie, nil
		}
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return thirdparty.Account{}, "", err
		}
	}
	return thirdparty.Account{}, "", secrets.ErrNotFound
}

func (m *accountUsageManager) prepareCookie(ctx context.Context, cookie string) (string, error) {
	if m.session == nil {
		return cookie, nil
	}
	prepared, err := m.session.PrepareCookie(ctx, cookie)
	if err != nil {
		return "", err
	}
	if prepared.Cookie != "" && prepared.Cookie != cookie && (prepared.Refreshed || prepared.Enriched) {
		return prepared.Cookie, nil
	}
	return cookie, nil
}
