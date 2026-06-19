package httpaction

import "context"

type CredentialInjector interface {
	Inject(context.Context, CredentialRequest) (CredentialResult, error)
}

type CredentialRequest struct {
	PluginID   string
	RawURL     string
	ScopeHosts []string
	Headers    map[string]string
}

type CredentialResult struct {
	URL          string
	AfterSuccess func(context.Context) error
}
