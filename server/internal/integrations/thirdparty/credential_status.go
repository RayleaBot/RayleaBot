package thirdparty

import "time"

// CheckedCredential describes an observed authentication result. Transport,
// upstream and risk-control errors leave validity unknown; only an explicit
// authentication rejection invalidates credentials.
func CheckedCredential(platform string, checkedAt time.Time, kind ErrorKind) CredentialStatus {
	checkedAt = checkedAt.UTC()
	status := CredentialStatus{State: CredentialValid, CheckedAt: &checkedAt}
	if kind == "" {
		return status
	}
	label := platform
	switch platform {
	case PlatformBilibili:
		label = "Bilibili"
	case PlatformWeibo:
		label = "微博"
	case PlatformDouyin:
		label = "抖音"
	case PlatformNeteaseMusic:
		label = "网易云音乐"
	}
	status.State = CredentialUnknown
	switch kind {
	case ErrorAuth, ErrorExpired:
		status.State = CredentialInvalid
		if platform == PlatformBilibili {
			label += " "
		}
		status.LastError = label + "账号 CK 已失效，请重新扫码"
	case ErrorRiskControl, ErrorCaptcha:
		status.LastError = label + " CK 检查受到平台风控限制，请稍后重试"
	case ErrorRateLimit:
		status.LastError = label + " CK 检查触发频率限制，请稍后重试"
	default:
		status.LastError = label + " CK 状态暂时无法确认，请稍后重试"
	}
	return status
}
