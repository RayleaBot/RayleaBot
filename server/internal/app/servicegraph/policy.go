package servicegraph

import (
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

type policyRepositories struct {
	Blacklist      permission.BlacklistRepository
	Whitelist      permission.WhitelistRepository
	WhitelistState permission.WhitelistStateRepository
}

func buildPolicyRepositories(platform appplatform.State) policyRepositories {
	return policyRepositories{
		Blacklist:      permission.NewSQLiteBlacklistRepository(platform.Storage.Read, platform.Storage.Write),
		Whitelist:      permission.NewSQLiteWhitelistRepository(platform.Storage.Read, platform.Storage.Write),
		WhitelistState: permission.NewSQLiteWhitelistStateRepository(platform.Storage.Read, platform.Storage.Write),
	}
}
