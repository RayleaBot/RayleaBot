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
