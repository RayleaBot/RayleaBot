package bilibili

import (
	"context"
	"net/http"
	"time"

	bilibilicredential "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/credential"
	bilibilimonitoring "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/monitoring"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type Source = bilibilisource.Source
type Store = bilibilisource.Store
type SourceStatus = bilibilisource.Status
type Dispatcher = bilibilisource.Dispatcher
type BilibiliEvent = bilibilisource.BilibiliEvent
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

func RuntimeEvent(event BilibiliEvent, timestamp int64) runtimeprotocol.Event {
	return runtimeprotocol.Event{
		EventID:        event.EventType + ":" + event.UID + ":" + event.ID,
		SourceProtocol: bilibilisource.SourceProtocol,
		SourceAdapter:  bilibilisource.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      timestamp,
		PayloadFields: map[string]any{
			"bilibili": bilibilimonitoring.Payload(event),
		},
	}
}

type EventDispatcherFunc func(context.Context, BilibiliEvent, int64)

func (f EventDispatcherFunc) DispatchBilibiliEvent(ctx context.Context, event BilibiliEvent, timestamp int64) {
	if f != nil {
		f(ctx, event, timestamp)
	}
}
