package managementhttp

import (
	"testing"
	"time"

	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/bilibili/source"
)

func TestBilibiliSourceStatusResponseIncludesDiagnosis(t *testing.T) {
	t.Parallel()

	retryAt := time.Date(2026, 6, 8, 8, 35, 0, 0, time.UTC)
	status := bilibilisource.Status{
		Status:  bilibilisource.StateDegraded,
		Summary: "Bilibili 事件源运行受限",
		Live: bilibilisource.LiveStatus{
			WatchedRooms:    1,
			FallbackPolling: true,
			LastError:       "code -352",
		},
		Dynamic: bilibilisource.DynamicStatus{
			Enabled:         true,
			IntervalSeconds: 10,
			WatchedUIDs:     1,
		},
		Diagnosis: bilibilisource.Diagnosis{
			Level:       "attention",
			Headline:    "平台风控等待中",
			Description: "Bilibili 暂时限制部分请求，系统会在等待结束后自动恢复检查。",
			Causes: []bilibilisource.DiagnosisCause{
				{
					Scope:     "live",
					Code:      "platform_risk_control",
					Title:     "直播请求被平台限制",
					Detail:    "直播状态检查暂时等待平台恢复。",
					LastError: "code -352",
					RetryAt:   &retryAt,
				},
			},
			Impacts:   []string{"动态接收不受影响。", "CK 有效，无需重新登录。"},
			Actions:   []bilibilisource.DiagnosisAction{{Kind: "wait", Label: "等待平台恢复", Primary: true}},
			UpdatedAt: time.Date(2026, 6, 8, 8, 30, 0, 0, time.UTC),
		},
	}

	response := bilibiliSourceStatusResponseFrom(status)
	if response.Diagnosis.Level != "attention" || response.Diagnosis.Headline != "平台风控等待中" {
		t.Fatalf("unexpected diagnosis response: %#v", response.Diagnosis)
	}
	if len(response.Diagnosis.Causes) != 1 || response.Diagnosis.Causes[0].RetryAt == nil || *response.Diagnosis.Causes[0].RetryAt != "2026-06-08T08:35:00Z" {
		t.Fatalf("unexpected diagnosis causes: %#v", response.Diagnosis.Causes)
	}
	if len(response.Diagnosis.Actions) != 1 || response.Diagnosis.Actions[0].Kind != "wait" {
		t.Fatalf("unexpected diagnosis actions: %#v", response.Diagnosis.Actions)
	}
}
