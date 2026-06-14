package source

import (
	"context"

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
	if isBilibiliAuthError(err) {
		checkedAt := s.now()
		_ = s.accounts.UpdateCredentialStatus(ctx, account.Platform, account.AccountID, account.Profile, thirdparty.CredentialStatus{
			State:     thirdparty.CredentialInvalid,
			CheckedAt: &checkedAt,
			LastError: err.Error(),
		})
		s.stopRoomTasks()
		return accountRequestErrorAuth
	}
	if bilibErr := asBilibiliError(err); bilibErr != nil && bilibErr.Kind == ErrorCaptcha {
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
	biliErr := asBilibiliError(err)
	if biliErr == nil {
		return
	}
	vVoucher := ExtractVVoucher([]byte(biliErr.Body))
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
