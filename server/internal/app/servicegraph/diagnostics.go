package servicegraph

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph/integrationmodule"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

type thirdPartyDiagnostics struct {
	service *integrationmodule.ThirdPartyService
}

func (d thirdPartyDiagnostics) DiagnosticsThirdParty(ctx context.Context) (systemsvc.DiagnosticsThirdParty, []health.DiagnosticIssue) {
	result := systemsvc.DiagnosticsThirdParty{
		Platforms: []systemsvc.DiagnosticsThirdPartyPlatform{},
	}
	platforms := map[string]*systemsvc.DiagnosticsThirdPartyPlatform{}
	for _, platform := range thirdparty.SupportedPlatforms() {
		platforms[platform] = &systemsvc.DiagnosticsThirdPartyPlatform{Platform: platform}
	}
	if d.service == nil {
		result.Platforms = sortedThirdPartyPlatforms(platforms)
		return result, nil
	}
	accounts, err := d.service.List(ctx)
	if err != nil {
		result.Platforms = sortedThirdPartyPlatforms(platforms)
		return result, []health.DiagnosticIssue{{
			Code:        "third_party.accounts_unavailable",
			Severity:    "warning",
			Summary:     "第三方账号状态不可读",
			Remediation: "请检查数据库连接和 third_party_accounts 表状态。",
		}}
	}
	for _, account := range accounts {
		platform := strings.TrimSpace(account.Platform)
		if platform == "" {
			continue
		}
		item := platforms[platform]
		if item == nil {
			item = &systemsvc.DiagnosticsThirdPartyPlatform{Platform: platform}
			platforms[platform] = item
		}
		result.Total++
		item.Total++
		if account.Enabled {
			result.Enabled++
			item.Enabled++
		}
		if account.Configured {
			result.Configured++
			item.Configured++
		}
		if account.Credential.State == thirdparty.CredentialInvalid {
			result.Invalid++
			item.Invalid++
		}
	}
	result.Platforms = sortedThirdPartyPlatforms(platforms)
	return result, nil
}

func sortedThirdPartyPlatforms(platforms map[string]*systemsvc.DiagnosticsThirdPartyPlatform) []systemsvc.DiagnosticsThirdPartyPlatform {
	keys := make([]string, 0, len(platforms))
	for key := range platforms {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]systemsvc.DiagnosticsThirdPartyPlatform, 0, len(keys))
	for _, key := range keys {
		items = append(items, *platforms[key])
	}
	return items
}

type bilibiliSourceDiagnostics struct {
	source *integrationmodule.BilibiliSource
}

func (d bilibiliSourceDiagnostics) DiagnosticsBilibiliSource(ctx context.Context) (systemsvc.DiagnosticsBilibiliSource, []health.DiagnosticIssue) {
	result := systemsvc.DiagnosticsBilibiliSource{
		Status: "disabled",
		Issues: []health.DiagnosticIssue{},
	}
	if d.source == nil {
		result.Summary = "Bilibili 事件源未启用"
		return result, nil
	}
	status := d.source.Status(ctx)
	result.Status = status.Status
	result.Summary = status.Summary
	result.DiagnosisLevel = status.Diagnosis.Level
	result.WatchedRooms = status.Live.WatchedRooms
	result.WatchedUIDs = status.Dynamic.WatchedUIDs
	if status.Live.LastEventAt != nil {
		result.LiveLastEventAt = status.Live.LastEventAt.UTC().Format(time.RFC3339)
	}
	if status.Dynamic.LastPollAt != nil {
		result.DynamicLastPollAt = status.Dynamic.LastPollAt.UTC().Format(time.RFC3339)
	}
	for _, cause := range status.Diagnosis.Causes {
		if cause.Code == "" || cause.Code == "healthy" {
			continue
		}
		summary := strings.TrimSpace(cause.Title)
		if summary == "" {
			summary = strings.TrimSpace(cause.Detail)
		}
		remediation := "请在 Bilibili 事件源状态页查看详情，并按建议刷新或重启事件源。"
		if len(status.Diagnosis.Actions) > 0 {
			remediation = status.Diagnosis.Actions[0].Label
		}
		result.Issues = append(result.Issues, health.DiagnosticIssue{
			Code:        "bilibili_source." + cause.Code,
			Severity:    bilibiliDiagnosisSeverity(status.Diagnosis.Level),
			Summary:     summary,
			Remediation: remediation,
		})
	}
	if result.Issues == nil {
		result.Issues = []health.DiagnosticIssue{}
	}
	return result, nil
}

func bilibiliDiagnosisSeverity(level string) string {
	switch level {
	case "action_required":
		return "error"
	case "attention":
		return "warning"
	default:
		return "warning"
	}
}

type schedulerDiagnostics struct {
	scheduler *scheduler.Engine
}

func (d schedulerDiagnostics) DiagnosticsScheduler() systemsvc.DiagnosticsScheduler {
	result := systemsvc.DiagnosticsScheduler{}
	if d.scheduler == nil {
		return result
	}
	now := time.Now().UTC()
	result.Running = d.scheduler.RunningCount()
	for _, job := range d.scheduler.Jobs() {
		result.Total++
		if job.Enabled {
			result.Enabled++
			if !job.NextRun.After(now) {
				result.Pending++
			}
		} else {
			result.Disabled++
		}
		if job.LastError != nil {
			result.Failed++
		}
	}
	return result
}
