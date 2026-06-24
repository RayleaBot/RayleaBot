package bilibili

import (
	"net/http"
	"time"

	bilibilicredential "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/credential"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type Source = bilibilisource.Source
type Store = bilibilisource.Store
type SourceStatus = bilibilisource.Status
type Dispatcher = bilibilisource.Dispatcher
type AccountClient = bilibilisession.AccountClient
type HTTPCredentialInjector = bilibilicredential.Injector

type Deps struct {
	Store         Store
	Accounts      *thirdparty.Service
	PluginConfig  bilibilisubscriptions.PluginConfigReader
	Dispatcher    Dispatcher
	NotifyStatus  func(SourceStatus)
	HTTPTransport http.RoundTripper
	Now           func() time.Time
}

type Module struct {
	Source          *Source
	AccountClient   *AccountClient
	HTTPCredentials *HTTPCredentialInjector
}

func Build(deps Deps) (Module, error) {
	sessionClient := bilibilisession.NewSessionClient(deps.HTTPTransport, deps.Now, nil)
	source, err := bilibilisource.NewSource(bilibilisource.Deps{
		Store:         deps.Store,
		Accounts:      deps.Accounts,
		Subjects:      bilibilisubscriptions.NewPluginConfigProvider(deps.PluginConfig),
		Dispatcher:    deps.Dispatcher,
		NotifyStatus:  deps.NotifyStatus,
		HTTPTransport: deps.HTTPTransport,
		Session:       sessionClient,
		Now:           deps.Now,
	})
	if err != nil {
		return Module{}, err
	}
	return Module{
		Source:          source,
		AccountClient:   bilibilisession.NewAccountClient(deps.HTTPTransport, deps.Now, nil),
		HTTPCredentials: bilibilicredential.NewInjector(deps.Accounts, sessionClient),
	}, nil
}
