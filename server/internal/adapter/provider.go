package adapter

import adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"

const (
	ProviderUnknown     = adaptershell.ProviderUnknown
	ProviderStandard    = adaptershell.ProviderStandard
	ProviderNapCat      = adaptershell.ProviderNapCat
	ProviderLuckyLillia = adaptershell.ProviderLuckyLillia
)

func DetectProvider(appName string) string {
	return adaptershell.DetectProvider(appName)
}
