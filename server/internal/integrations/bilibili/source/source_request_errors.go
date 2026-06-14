package source

import (
	"context"

	bilibiliCaptcha "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/captcha"
	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type accountRequestErrorAction int

const (
	accountRequestErrorNone accountRequestErrorAction = iota
	accountRequestErrorAuth
	accountRequestErrorCooldown
)

func (s *Source) handleAccountRequestError(ctx context.Context, account thirdparty.Account, cookie, scope string, err error) accountRequestErrorAction {
	if err == nil || account.Platform == "" || account.AccountID == "" {
		return accountRequestErrorNone
	}
	if bilibiliSession.IsAuthError(err) {
		checkedAt := s.now()
		_ = s.accounts.UpdateCredentialStatus(ctx, account.Platform, account.AccountID, account.Profile, thirdparty.CredentialStatus{
			State:     thirdparty.CredentialInvalid,
			CheckedAt: &checkedAt,
			LastError: err.Error(),
		})
		s.stopRoomTasks()
		return accountRequestErrorAuth
	}
	if bilibErr := bilibiliSession.AsError(err); bilibErr != nil && bilibErr.Kind == bilibiliSession.ErrorCaptcha {
		s.rememberRequestCooldown(scope, account, cookie, err)
		go s.tryCaptchaRecovery(ctx, account, cookie, err)
		return accountRequestErrorCooldown
	}
	if isBilibiliRequestCooldownError(err) {
		s.rememberRequestCooldown(scope, account, cookie, err)
		return accountRequestErrorCooldown
	}
	return accountRequestErrorNone
}

func (s *Source) tryCaptchaRecovery(ctx context.Context, account thirdparty.Account, cookie string, err error) {
	biliErr := bilibiliSession.AsError(err)
	if biliErr == nil {
		return
	}
	vVoucher := bilibiliCaptcha.ExtractVVoucher([]byte(biliErr.Body))
	if vVoucher == "" {
		return
	}
	result, solveErr := s.captchaClient.TrySolve(ctx, vVoucher, cookie)
	if solveErr != nil {
		return
	}
	s.griskMu.Lock()
	s.griskID = result.GriskID
	s.griskMu.Unlock()
}
