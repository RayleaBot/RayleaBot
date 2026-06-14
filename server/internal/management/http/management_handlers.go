package managementhttp

type ManagementHandlers struct {
	auth            managementAuthService
	system          managementSystemService
	requestShutdown func()
}

type ManagementDeps struct {
	Auth            managementAuthService
	System          managementSystemService
	RequestShutdown func()
}

func NewManagementHandlers(deps ManagementDeps) *ManagementHandlers {
	return &ManagementHandlers{
		auth:            deps.Auth,
		system:          deps.System,
		requestShutdown: deps.RequestShutdown,
	}
}

func (h *ManagementHandlers) SetAuthManager(auth managementAuthService) {
	if h == nil {
		return
	}
	h.auth = auth
}

type managementAuthService interface {
	IsBootstrapped() bool
	Revoke(string) error
}

type managementSystemService interface {
	ManagementStatusSnapshot() SystemStatusResponse
	PublishStatusSnapshot()
}
