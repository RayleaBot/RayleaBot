package wsevents

import (
	"context"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

// OneBot11Ingress is the inbound surface of one configured OneBot adapter. The
// two OneBot transports that the peer dials into are addressed per instance,
// because several instances can be listening at once and each has its own
// credential.
type OneBot11Ingress struct {
	shell    *onebot11.Shell
	settings config.OneBotConfig
}

// OneBot11Ingress returns the ingress of the OneBot adapter with this id. The
// second result is false when no such adapter is running, which is how a route
// answers an unknown id or an adapter speaking another protocol.
func (s *ProtocolService) OneBot11Ingress(id string) (OneBot11Ingress, bool) {
	if s == nil || s.config == nil {
		return OneBot11Ingress{}, false
	}
	shell := s.oneBotShell(id)
	if shell == nil {
		return OneBot11Ingress{}, false
	}
	settings, ok := s.config.CurrentConfig().OneBot11Settings(id)
	if !ok {
		return OneBot11Ingress{}, false
	}
	return OneBot11Ingress{shell: shell, settings: settings}, true
}

func (i OneBot11Ingress) ReverseWSEnabled() bool {
	return i.transportEnabled(onebot11.TransportReverseWS)
}

func (i OneBot11Ingress) ReverseWSAccessToken() string {
	return i.settings.ReverseWS.AccessToken
}

func (i OneBot11Ingress) ReverseWSAccessTokenQueryCompat() bool {
	return i.settings.ReverseWS.AccessTokenQueryCompat
}

func (i OneBot11Ingress) MarkReverseWSAuthFailed() {
	i.shell.MarkReverseWSAuthFailed()
}

func (i OneBot11Ingress) AttachReverseWS(conn *websocket.Conn) {
	i.shell.AttachReverseWS(conn)
}

func (i OneBot11Ingress) WebhookEnabled() bool {
	return i.transportEnabled(onebot11.TransportWebhook)
}

func (i OneBot11Ingress) WebhookAccessToken() string {
	return i.settings.Webhook.AccessToken
}

func (i OneBot11Ingress) WebhookAccessTokenQueryCompat() bool {
	return i.settings.Webhook.AccessTokenQueryCompat
}

func (i OneBot11Ingress) MarkWebhookAuthFailed() {
	i.shell.MarkWebhookAuthFailed()
}

func (i OneBot11Ingress) AcceptWebhookPayload(ctx context.Context, payload []byte) error {
	return i.shell.AcceptWebhookPayload(ctx, payload)
}

func (i OneBot11Ingress) transportEnabled(transport onebot11.TransportKey) bool {
	snapshot := i.shell.Snapshot()
	switch transport {
	case onebot11.TransportReverseWS:
		return snapshot.ReverseWS.Enabled && snapshot.ReverseWS.Configured
	case onebot11.TransportWebhook:
		return snapshot.Webhook.Enabled && snapshot.Webhook.Configured
	default:
		return false
	}
}

func (s *ProtocolService) oneBotShell(id string) *onebot11.Shell {
	if s == nil {
		return nil
	}
	// A typed nil in the map would pass an interface nil check later, so the
	// concrete pointer is what is tested here.
	shell, ok := s.oneBotShells[id]
	if !ok || shell == nil {
		return nil
	}
	return shell
}

func (s *ProtocolService) qqClient(id string) QQOfficialStatusSource {
	if s == nil {
		return nil
	}
	client, ok := s.qqClients[id]
	if !ok || client == nil {
		return nil
	}
	return client
}
