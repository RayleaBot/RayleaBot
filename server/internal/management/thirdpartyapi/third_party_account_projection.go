package thirdpartyapi

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type thirdPartyAccountsResponse struct {
	Items []thirdPartyAccountSummary `json:"items"`
}

type thirdPartyAccountUpsertRequest struct {
	Label        *string                   `json:"label"`
	Enabled      *bool                     `json:"enabled"`
	Cookie       string                    `json:"cookie,omitempty"`
	Profile      *thirdPartyAccountProfile `json:"profile,omitempty"`
	ProxyURL     *string                   `json:"proxy_url,omitempty"`
	ProxyEnabled *bool                     `json:"proxy_enabled,omitempty"`
}

type thirdPartyAccountUpsertResponse struct {
	Account thirdPartyAccountSummary `json:"account"`
}

type thirdPartyAccountSummary struct {
	Platform     string                         `json:"platform"`
	AccountID    string                         `json:"account_id"`
	Label        string                         `json:"label"`
	Enabled      bool                           `json:"enabled"`
	Configured   bool                           `json:"configured"`
	Profile      *thirdPartyAccountProfile      `json:"profile"`
	Credential   thirdPartyCredentialStatus     `json:"credential"`
	Polling      thirdPartyAccountPollingStatus `json:"polling"`
	ProxyURL     string                         `json:"proxy_url"`
	ProxyEnabled bool                           `json:"proxy_enabled"`
	UpdatedAt    string                         `json:"updated_at"`
}

type thirdPartyAccountProfile struct {
	UID       string `json:"uid"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

func (profile *thirdPartyAccountProfile) accountProfile() thirdparty.AccountProfile {
	if profile == nil {
		return thirdparty.AccountProfile{}
	}
	return thirdparty.AccountProfile{
		UID:       profile.UID,
		Nickname:  profile.Nickname,
		AvatarURL: profile.AvatarURL,
	}
}

type thirdPartyCredentialStatus struct {
	State     string  `json:"state"`
	CheckedAt *string `json:"checked_at"`
	LastError string  `json:"last_error"`
}

type thirdPartyAccountPollingStatus struct {
	Enabled    bool    `json:"enabled"`
	LastUsedAt *string `json:"last_used_at"`
}

func accountSummaries(accounts []thirdparty.Account) []thirdPartyAccountSummary {
	items := make([]thirdPartyAccountSummary, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, accountSummary(account))
	}
	return items
}

func accountSummary(account thirdparty.Account) thirdPartyAccountSummary {
	var profile *thirdPartyAccountProfile
	if strings.TrimSpace(account.Profile.UID) != "" || strings.TrimSpace(account.Profile.Nickname) != "" || strings.TrimSpace(account.Profile.AvatarURL) != "" {
		profile = &thirdPartyAccountProfile{
			UID:       account.Profile.UID,
			Nickname:  account.Profile.Nickname,
			AvatarURL: account.Profile.AvatarURL,
		}
	}
	return thirdPartyAccountSummary{
		Platform:   account.Platform,
		AccountID:  account.AccountID,
		Label:      account.Label,
		Enabled:    account.Enabled,
		Configured: account.Configured,
		Profile:    profile,
		Credential: thirdPartyCredentialStatus{
			State:     account.Credential.State,
			CheckedAt: timeStringPtr(account.Credential.CheckedAt),
			LastError: account.Credential.LastError,
		},
		Polling: thirdPartyAccountPollingStatus{
			Enabled:    account.Enabled && account.Configured,
			LastUsedAt: timeStringPtr(account.LastUsedAt),
		},
		ProxyURL:     account.ProxyURL,
		ProxyEnabled: account.ProxyEnabled,
		UpdatedAt:    timeString(account.UpdatedAt),
	}
}
