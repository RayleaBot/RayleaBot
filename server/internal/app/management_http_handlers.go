package app

type managementHTTPHandlers struct {
	auth            managementAuthService
	system          managementSystemService
	requestShutdown func()
}

func newManagementHTTPHandlers(deps managementHTTPDeps) *managementHTTPHandlers {
	return &managementHTTPHandlers{
		auth:            deps.auth,
		system:          deps.system,
		requestShutdown: deps.requestShutdown,
	}
}

type managementAuthService interface {
	IsBootstrapped() bool
	Revoke(string) error
}

type managementSystemService interface {
	managementStatusSnapshot() systemStatusResponse
	publishStatusSnapshot()
}
