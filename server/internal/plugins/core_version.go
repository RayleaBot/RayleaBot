package plugins

import (
	"fmt"

	semverutil "github.com/RayleaBot/RayleaBot/server/internal/platform/semver"
)

type CoreVersionIncompatibilityReason string

const (
	CoreVersionUnknown CoreVersionIncompatibilityReason = "core_version_unknown"
	CoreVersionTooOld  CoreVersionIncompatibilityReason = "core_version_too_old"
)

type CoreVersionIncompatibleError struct {
	Reason         CoreVersionIncompatibilityReason
	CoreVersion    string
	MinCoreVersion string
}

func CheckCoreVersion(coreVersion, minCoreVersion string) *CoreVersionIncompatibleError {
	var reason CoreVersionIncompatibilityReason
	switch {
	case coreVersion == "unknown":
		reason = CoreVersionUnknown
	case semverutil.Compare(coreVersion, minCoreVersion) < 0:
		reason = CoreVersionTooOld
	default:
		return nil
	}
	return &CoreVersionIncompatibleError{
		Reason:         reason,
		CoreVersion:    coreVersion,
		MinCoreVersion: minCoreVersion,
	}
}

func (e *CoreVersionIncompatibleError) Error() string {
	if e.Reason == CoreVersionUnknown {
		return "无法确认当前 RayleaBot 版本，不能安装要求最低版本的插件。"
	}
	return fmt.Sprintf("插件要求 RayleaBot %s 或更高版本，当前版本为 %s", e.MinCoreVersion, e.CoreVersion)
}

func (e *CoreVersionIncompatibleError) Details() map[string]any {
	return map[string]any{
		"incompatible_reason": string(e.Reason),
		"min_core_version":    e.MinCoreVersion,
	}
}
