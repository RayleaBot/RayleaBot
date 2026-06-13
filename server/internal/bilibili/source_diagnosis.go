package bilibili

import (
	"fmt"
	"time"
)

func diagnosisForStatusAt(status Status, cooldowns []requestCooldown, now time.Time) Diagnosis {
	status.Status = normalizeSourceState(status.Status)
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
