package app

import "strings"

// botIdentityProvider is one adapter's view of who it is logged in as.
type botIdentityProvider func() string

// botIdentitySource reports the bot identity the plugin runtime works with.
//
// The runtime models a single identity, while adapters each authenticate in
// their own namespace and the project does not merge identities across them.
// Until the plugin protocol carries one identity per adapter, this reports the
// first connected adapter's, in configuration order, so a deployment that runs
// only a QQ adapter still has an identity instead of none.
type botIdentitySource struct {
	providers []botIdentityProvider
}

func (s botIdentitySource) CurrentBotID() string {
	for _, provider := range s.providers {
		if id := strings.TrimSpace(provider()); id != "" {
			return id
		}
	}
	return ""
}
