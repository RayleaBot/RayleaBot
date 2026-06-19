package diagnostics

import (
	"fmt"
	"strings"
	"time"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	StateDisabled   = "disabled"
	StateIdle       = "idle"
	StateConnecting = "connecting"
	StateConnected  = "connected"
	StateDegraded   = "degraded"
	StateFailed     = "failed"

	CooldownScopeLive       = "live"
	CooldownScopeDynamic    = "dynamic"
	CooldownScopeAutoFollow = "auto_follow"
)

type Status struct {
	Status    string               `json:"status"`
	Summary   string               `json:"summary"`
	Live      LiveStatus           `json:"live"`
	Dynamic   DynamicStatus        `json:"dynamic"`
	Diagnosis Diagnosis            `json:"diagnosis"`
	Accounts  []thirdparty.Account `json:"-"`
}

type LiveStatus struct {
	WatchedRooms    int        `json:"watched_rooms"`
	ConnectedRooms  int        `json:"connected_rooms"`
	FailedRooms     int        `json:"failed_rooms"`
	FallbackPolling bool       `json:"fallback_polling"`
	LastEventAt     *time.Time `json:"last_event_at"`
	LastError       string     `json:"last_error"`
}

type DynamicStatus struct {
	Enabled         bool       `json:"enabled"`
	IntervalSeconds int        `json:"interval_seconds"`
	WatchedUIDs     int        `json:"watched_uids"`
	AutoFollow      bool       `json:"auto_follow"`
	LastPollAt      *time.Time `json:"last_poll_at"`
	LastEventAt     *time.Time `json:"last_event_at"`
	LastError       string     `json:"last_error"`
}

type Diagnosis struct {
	Level       string            `json:"level"`
	Headline    string            `json:"headline"`
	Description string            `json:"description"`
	Causes      []DiagnosisCause  `json:"causes"`
	Impacts     []string          `json:"impacts"`
	Actions     []DiagnosisAction `json:"actions"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type DiagnosisCause struct {
	Scope     string     `json:"scope"`
	Code      string     `json:"code"`
	Title     string     `json:"title"`
	Detail    string     `json:"detail"`
	LastError string     `json:"last_error"`
	RetryAt   *time.Time `json:"retry_at"`
}

type DiagnosisAction struct {
	Kind    string  `json:"kind"`
	Label   string  `json:"label"`
	Target  *string `json:"target"`
	Primary bool    `json:"primary"`
}

type Cooldown struct {
	Attempts  int
	Until     time.Time
	LastError string
	Scope     string
	Code      string
}

func ForStatus(status Status, cooldowns []Cooldown, now time.Time) Diagnosis {
	status.Status = NormalizeState(status.Status)
	now = now.UTC()
	diagnosis := Diagnosis{
		Level:     "normal",
		Headline:  "Bilibili 事件源运行中",
		UpdatedAt: now,
		Causes:    []DiagnosisCause{},
		Impacts:   []string{},
		Actions: []DiagnosisAction{
			{Kind: "refresh", Label: "刷新状态", Primary: true},
		},
	}

	if status.Status == StateDisabled {
		diagnosis.Headline = "Bilibili 事件源未启用"
		diagnosis.Description = "启用订阅后，直播和动态状态会开始检查。"
		diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
			Scope:  "source",
			Code:   "source_disabled",
			Title:  "事件源未启用",
			Detail: "当前没有启用 Bilibili 事件源。",
		})
		diagnosis.Impacts = []string{"直播状态未检查。", "动态状态未检查。"}
		return diagnosis
	}

	if invalid := invalidCredentialCause(status.Accounts); invalid != nil {
		diagnosis.Level = "action_required"
		diagnosis.Headline = "CK 需要重新登录"
		diagnosis.Description = "Bilibili CK 无效，直播和动态检查需要可用 CK。"
		diagnosis.Causes = append(diagnosis.Causes, *invalid)
		diagnosis.Impacts = []string{"直播状态无法可靠检查。", "动态接收会受影响。", "需要重新获取 Bilibili CK。"}
		diagnosis.Actions = []DiagnosisAction{
			{Kind: "open_accounts", Label: "查看 Bilibili CK", Target: stringPtr("/third-party-accounts"), Primary: true},
			{Kind: "refresh", Label: "刷新状态", Primary: false},
		}
		return diagnosis
	}

	for _, cooldown := range cooldowns {
		diagnosis.Causes = append(diagnosis.Causes, cooldownCause(cooldown))
	}
	if len(cooldowns) > 0 {
		diagnosis.Level = "attention"
		diagnosis.Headline = "平台风控等待中"
		diagnosis.Description = "Bilibili 暂时限制部分请求，系统会在等待结束后自动恢复检查。"
		diagnosis.Impacts = cooldownImpacts(cooldowns, status)
		diagnosis.Actions = []DiagnosisAction{
			{Kind: "wait", Label: "等待平台恢复", Primary: true},
			{Kind: "refresh", Label: "刷新状态", Primary: false},
		}
		return diagnosis
	}

	if status.Status == StateIdle {
		diagnosis.Headline = "等待监控目标"
		diagnosis.Description = "当前没有可检查的 Bilibili 直播或动态目标。"
		diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
			Scope:  "source",
			Code:   "source_idle",
			Title:  "没有监控目标",
			Detail: "配置订阅目标后，事件源会开始检查直播和动态。",
		})
		diagnosis.Impacts = []string{"直播状态未检查。", "动态状态未检查。"}
		return diagnosis
	}

	if status.Live.FailedRooms > 0 && status.Live.FallbackPolling {
		diagnosis.Level = "attention"
		diagnosis.Headline = "直播备用检查中"
		diagnosis.Description = "部分直播长连接不可用，系统正在使用接口检查直播状态。"
		diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
			Scope:     "live",
			Code:      "live_fallback",
			Title:     "直播实时连接受限",
			Detail:    fmt.Sprintf("%d 个直播间未建立实时连接，开播与下播会通过备用接口继续检查。", status.Live.FailedRooms),
			LastError: status.Live.LastError,
		})
		diagnosis.Impacts = []string{"直播状态仍会检查，但实时性可能降低。", dynamicImpact(status), accountImpact(status.Accounts)}
		diagnosis.Actions = []DiagnosisAction{
			{Kind: "restart_source", Label: "重启事件源", Primary: true},
			{Kind: "refresh", Label: "刷新状态", Primary: false},
		}
		return diagnosis
	}

	if status.Live.LastError != "" || status.Dynamic.LastError != "" || status.Status == StateFailed {
		diagnosis.Level = "action_required"
		diagnosis.Headline = "Bilibili 事件源需要处理"
		diagnosis.Description = "事件源存在检查错误，查看原因后刷新或重启事件源。"
		if status.Live.LastError != "" {
			diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
				Scope:     "live",
				Code:      "live_connection_error",
				Title:     "直播检查异常",
				Detail:    "直播检查遇到错误。",
				LastError: status.Live.LastError,
			})
		}
		if status.Dynamic.LastError != "" {
			diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
				Scope:     "dynamic",
				Code:      "source_failed",
				Title:     "动态检查异常",
				Detail:    "动态检查遇到错误。",
				LastError: status.Dynamic.LastError,
			})
		}
		diagnosis.Impacts = []string{liveImpact(status), dynamicImpact(status), accountImpact(status.Accounts)}
		diagnosis.Actions = []DiagnosisAction{
			{Kind: "restart_source", Label: "重启事件源", Primary: true},
			{Kind: "refresh", Label: "刷新状态", Primary: false},
		}
		return diagnosis
	}

	if status.Status == StateConnecting {
		diagnosis.Level = "attention"
		diagnosis.Headline = "正在连接 Bilibili 事件源"
		diagnosis.Description = "直播和动态检查正在恢复，稍后刷新状态。"
		diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
			Scope:  "source",
			Code:   "source_connecting",
			Title:  "事件源正在连接",
			Detail: "直播连接、备用检查和动态检查正在启动。",
		})
		diagnosis.Impacts = []string{"直播状态会在连接完成后更新。", "动态检查会在下一轮检查后更新。", accountImpact(status.Accounts)}
		return diagnosis
	}

	diagnosis.Headline = "Bilibili 事件源运行中"
	diagnosis.Description = "直播和动态检查正在正常运行。"
	diagnosis.Causes = append(diagnosis.Causes, DiagnosisCause{
		Scope:  "source",
		Code:   "healthy",
		Title:  "检查正常",
		Detail: "直播和动态检查正在按当前配置运行。",
	})
	diagnosis.Impacts = []string{liveImpact(status), dynamicImpact(status), accountImpact(status.Accounts)}
	return diagnosis
}

func Summary(state string) string {
	switch NormalizeState(state) {
	case StateDisabled:
		return "Bilibili 事件源未启用"
	case StateIdle:
		return "Bilibili 事件源等待订阅"
	case StateConnecting:
		return "Bilibili 事件源正在连接"
	case StateConnected:
		return "Bilibili 事件源运行中"
	case StateDegraded:
		return "Bilibili 事件源运行受限"
	case StateFailed:
		return "Bilibili 事件源连接失败"
	default:
		return "Bilibili 事件源状态未知"
	}
}

func NormalizeState(state string) string {
	switch state {
	case StateDisabled, StateIdle, StateConnecting, StateConnected, StateDegraded, StateFailed:
		return state
	default:
		return StateIdle
	}
}

func NormalizeCooldownScope(scope string) string {
	scope = strings.TrimSpace(scope)
	switch scope {
	case CooldownScopeLive, CooldownScopeDynamic, CooldownScopeAutoFollow:
		return scope
	default:
		if strings.HasPrefix(scope, CooldownScopeAutoFollow+":") {
			return CooldownScopeAutoFollow
		}
		return "source"
	}
}

func CooldownCode(err error) string {
	biliErr := bilibiliSession.AsError(err)
	if biliErr != nil && biliErr.Kind == bilibiliSession.ErrorRateLimit {
		return "platform_rate_limit"
	}
	return "platform_risk_control"
}

func invalidCredentialCause(accounts []thirdparty.Account) *DiagnosisCause {
	for _, account := range accounts {
		if account.Credential.State != thirdparty.CredentialInvalid {
			continue
		}
		detail := "账号 " + account.AccountID + " 的 CK 无效。"
		if strings.TrimSpace(account.Label) != "" {
			detail = account.Label + " 的 CK 无效。"
		}
		return &DiagnosisCause{
			Scope:     "account",
			Code:      "credential_invalid",
			Title:     "CK 无效",
			Detail:    detail,
			LastError: account.Credential.LastError,
		}
	}
	return nil
}

func cooldownCause(cooldown Cooldown) DiagnosisCause {
	scope := NormalizeCooldownScope(cooldown.Scope)
	title := "平台暂时限制请求"
	detail := "Bilibili 暂时限制部分请求，等待结束后会自动重试。"
	switch scope {
	case CooldownScopeLive:
		title = "直播请求被平台限制"
		detail = "直播状态检查暂时等待平台恢复。"
	case CooldownScopeDynamic:
		title = "动态请求被平台限制"
		detail = "动态检查暂时等待平台恢复。"
	case CooldownScopeAutoFollow:
		title = "自动关注请求被平台限制"
		detail = "自动关注暂时等待平台恢复。"
	}
	return DiagnosisCause{
		Scope:     scope,
		Code:      cooldown.Code,
		Title:     title,
		Detail:    detail,
		LastError: cooldown.LastError,
		RetryAt:   timePtr(cooldown.Until),
	}
}

func cooldownImpacts(cooldowns []Cooldown, status Status) []string {
	impacts := make([]string, 0, 4)
	hasLive := false
	hasDynamic := false
	hasAutoFollow := false
	for _, cooldown := range cooldowns {
		switch NormalizeCooldownScope(cooldown.Scope) {
		case CooldownScopeLive:
			hasLive = true
		case CooldownScopeDynamic:
			hasDynamic = true
		case CooldownScopeAutoFollow:
			hasAutoFollow = true
		}
	}
	if hasLive {
		impacts = append(impacts, "直播状态暂时等待平台恢复。")
	} else {
		impacts = append(impacts, liveImpact(status))
	}
	if hasDynamic {
		impacts = append(impacts, "动态检查暂时等待平台恢复。")
	} else {
		impacts = append(impacts, dynamicImpact(status))
	}
	if hasAutoFollow {
		impacts = append(impacts, "自动关注暂时等待平台恢复。")
	}
	impacts = append(impacts, accountImpact(status.Accounts))
	return impacts
}

func liveImpact(status Status) string {
	if status.Live.WatchedRooms == 0 {
		return "当前没有直播监控目标。"
	}
	if status.Live.FailedRooms > 0 {
		return "直播状态仍会检查，但实时性可能降低。"
	}
	return "直播状态正常检查。"
}

func dynamicImpact(status Status) string {
	if !status.Dynamic.Enabled || status.Dynamic.WatchedUIDs == 0 {
		return "当前没有动态监控目标。"
	}
	if status.Dynamic.LastError != "" {
		return "动态检查当前存在错误。"
	}
	return "动态接收不受影响。"
}

func accountImpact(accounts []thirdparty.Account) string {
	for _, account := range accounts {
		if account.Credential.State == thirdparty.CredentialInvalid {
			return "CK 需要重新登录。"
		}
	}
	return "CK 有效，无需重新登录。"
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}

func stringPtr(value string) *string {
	return &value
}
