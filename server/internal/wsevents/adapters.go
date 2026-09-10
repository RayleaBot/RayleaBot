package wsevents

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
	"github.com/RayleaBot/RayleaBot/server/internal/system"
)

// AdapterIdentity is the bot a connected adapter authenticates as.
type AdapterIdentity struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// AdapterDescriptor describes one configured adapter instance for the
// management surface. An instance exists because the operator added it, so the
// listing says what it is doing rather than whether it exists.
type AdapterDescriptor struct {
	ID          string                    `json:"id"`
	Protocol    string                    `json:"protocol"`
	DisplayName string                    `json:"display_name"`
	Enabled     bool                      `json:"enabled"`
	State       string                    `json:"state"`
	Summary     string                    `json:"summary"`
	Identity    *AdapterIdentity          `json:"identity,omitempty"`
	OneBot11    *OneBot11ProtocolSnapshot `json:"onebot11,omitempty"`
}

// AdapterProtocolDescriptor is a protocol the operator can add an instance of.
type AdapterProtocolDescriptor struct {
	Protocol    string `json:"protocol"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// AdaptersView is what the management surface needs to render the adapters
// page: the instances that exist, and the protocols one can be added for.
type AdaptersView struct {
	Adapters           []AdapterDescriptor         `json:"adapters"`
	AvailableProtocols []AdapterProtocolDescriptor `json:"available_protocols"`
}

// QQOfficialAdapter is the slice of the QQ client this package needs, so the
// service does not depend on the concrete adapter.
type QQOfficialAdapter interface {
	Status() qqofficial.Status
	// Reload applies new settings, reporting whether anything the connection
	// depends on changed.
	Reload(config.QQOfficialConfig) bool
	SetEnabled(bool)
}

var adapterProtocols = []AdapterProtocolDescriptor{
	{
		Protocol:    config.AdapterTypeOneBot11,
		DisplayName: "OneBot11",
		Description: "连接实现 OneBot11 的第三方客户端，支持反向 WebSocket、正向 WebSocket、HTTP API 与 Webhook 四种传输。",
	},
	{
		Protocol:    config.AdapterTypeQQOfficial,
		DisplayName: "QQ 官方机器人",
		Description: "通过 QQ 开放平台接入官方机器人，使用 AppID 与 AppSecret 鉴权。",
	},
}

// Adapters lists the configured adapter instances in configuration order,
// alongside the protocols an instance can be added for.
func (s *ProtocolService) Adapters() AdaptersView {
	view := AdaptersView{
		Adapters:           make([]AdapterDescriptor, 0, 2),
		AvailableProtocols: append([]AdapterProtocolDescriptor(nil), adapterProtocols...),
	}
	if s == nil || s.config == nil {
		return view
	}
	for _, instance := range s.config.CurrentConfig().Adapters {
		view.Adapters = append(view.Adapters, s.adapterDescriptor(instance))
	}
	return view
}

func (s *ProtocolService) adapterDescriptor(instance config.AdapterInstance) AdapterDescriptor {
	switch instance.Type {
	case config.AdapterTypeOneBot11:
		return s.oneBot11Descriptor(instance)
	case config.AdapterTypeQQOfficial:
		return s.qqOfficialDescriptor(instance)
	default:
		return AdapterDescriptor{
			ID:          instance.ID,
			Protocol:    instance.Type,
			DisplayName: instance.ID,
			Enabled:     instance.Enabled,
			State:       string(onebot11.StateStopped),
			Summary:     "适配器未启动。",
		}
	}
}

func (s *ProtocolService) oneBot11Descriptor(instance config.AdapterInstance) AdapterDescriptor {
	descriptor := AdapterDescriptor{
		ID:          instance.ID,
		Protocol:    config.AdapterTypeOneBot11,
		DisplayName: adapterDisplayName(instance, "OneBot11"),
		Enabled:     instance.Enabled,
		State:       string(onebot11.StateStopped),
		Summary:     "适配器未启动。",
	}
	shell := s.oneBotShell(instance.ID)
	if shell == nil {
		if !instance.Enabled {
			descriptor.Summary = "适配器已配置但未启用。"
		}
		return descriptor
	}
	snapshot := shell.Snapshot()
	details := oneBot11ProtocolSnapshot(snapshot)
	descriptor.OneBot11 = &details
	descriptor.State = string(snapshot.State)
	descriptor.Summary = oneBot11Summary(instance, snapshot)
	descriptor.Identity = oneBot11Identity(snapshot)
	return descriptor
}

func oneBot11Identity(snapshot onebot11.Snapshot) *AdapterIdentity {
	id := strings.TrimSpace(snapshot.BotID)
	name := ""
	for _, transport := range []onebot11.TransportSnapshot{snapshot.ReverseWS, snapshot.ForwardWS, snapshot.HTTPAPI} {
		if !transport.Enabled || transport.State != onebot11.TransportStateConnected {
			continue
		}
		info := transport.RuntimeInfo
		userID := strings.TrimSpace(info.UserID)
		if userID == "" {
			continue
		}
		// HTTP-only adapters can confirm their identity before receiving an event.
		if id == "" {
			id = userID
		}
		if userID == id && strings.TrimSpace(info.Nickname) != "" {
			name = strings.TrimSpace(info.Nickname)
			break
		}
	}
	if id == "" {
		return nil
	}
	identity := &AdapterIdentity{ID: id, Name: name}
	if strings.IndexFunc(id, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		identity.AvatarURL = oneBot11AvatarURL(id)
	}
	return identity
}

func (s *ProtocolService) qqOfficialDescriptor(instance config.AdapterInstance) AdapterDescriptor {
	descriptor := AdapterDescriptor{
		ID:          instance.ID,
		Protocol:    config.AdapterTypeQQOfficial,
		DisplayName: adapterDisplayName(instance, "QQ 官方机器人"),
		Enabled:     instance.Enabled,
		State:       qqofficial.StateIdle,
		Summary:     "适配器未启动。",
	}
	client := s.qqClient(instance.ID)
	if client == nil {
		if !instance.Enabled {
			descriptor.Summary = "适配器已配置但未启用。"
		}
		return descriptor
	}
	status := client.Status()
	descriptor.State = status.State
	descriptor.Summary = status.Summary
	if status.BotID != "" {
		descriptor.Identity = &AdapterIdentity{ID: status.BotID, Name: status.BotName, AvatarURL: status.BotAvatarURL}
	}
	return descriptor
}

// adapterDisplayName keeps several instances of one protocol apart: the first
// reads as the protocol itself, and any other carries its own identifier.
func adapterDisplayName(instance config.AdapterInstance, protocolName string) string {
	if instance.ID == config.DefaultOneBot11AdapterID || instance.ID == config.DefaultQQOfficialAdapterID {
		return protocolName
	}
	return protocolName + "（" + instance.ID + "）"
}

func oneBot11Summary(instance config.AdapterInstance, snapshot onebot11.Snapshot) string {
	// OneBot spreads its state across four transports: the adapter has work to
	// do once any one of them is both configured and turned on.
	configured := false
	for _, transport := range []onebot11.TransportSnapshot{
		snapshot.ReverseWS, snapshot.ForwardWS, snapshot.HTTPAPI, snapshot.Webhook,
	} {
		if transport.Configured {
			configured = true
		}
	}
	switch {
	case !configured:
		return "适配器未配置传输。"
	case !instance.Enabled:
		return "适配器已配置但未启用。"
	case snapshot.State == onebot11.StateConnected:
		return "已连接。"
	case snapshot.LastErrorMessage != "":
		return snapshot.LastErrorMessage
	default:
		return "等待连接。"
	}
}

// AdapterStates projects the same configured collection for status and diagnostics.
func (s *ProtocolService) AdapterStates() []system.AdapterStatus {
	descriptors := s.Adapters().Adapters
	states := make([]system.AdapterStatus, 0, len(descriptors))
	for _, descriptor := range descriptors {
		states = append(states, system.AdapterStatus{ID: descriptor.ID, Protocol: descriptor.Protocol, Enabled: descriptor.Enabled, State: descriptor.State})
	}
	return states
}
