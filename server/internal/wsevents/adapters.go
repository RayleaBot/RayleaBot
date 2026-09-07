package wsevents

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"

	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
)

// AdapterIdentity is the bot a connected adapter authenticates as.
type AdapterIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AdapterDescriptor describes one configured adapter instance for the
// management surface. An instance exists because the operator added it, so the
// listing says what it is doing rather than whether it exists.
type AdapterDescriptor struct {
	ID          string           `json:"id"`
	Protocol    string           `json:"protocol"`
	DisplayName string           `json:"display_name"`
	Enabled     bool             `json:"enabled"`
	State       string           `json:"state"`
	Summary     string           `json:"summary"`
	Identity    *AdapterIdentity `json:"identity,omitempty"`
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

// QQOfficialStatusSource is the slice of the QQ client this package needs, so
// the service does not depend on the concrete adapter for a status read.
type QQOfficialStatusSource interface {
	Status() qqofficial.Status
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
		Description: "连接 QQ 开放平台的官方机器人，使用 AppID 与 AppSecret 鉴权，消息标识位于该平台自己的命名空间。",
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
	descriptor.State = string(snapshot.State)
	descriptor.Summary = oneBot11Summary(instance, snapshot)
	if snapshot.BotID != "" {
		descriptor.Identity = &AdapterIdentity{ID: snapshot.BotID}
	}
	return descriptor
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
		descriptor.Identity = &AdapterIdentity{ID: status.BotID, Name: status.BotName}
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

func primaryQQOfficialOf(cfg config.Config) (config.AdapterInstance, config.QQOfficialConfig, bool) {
	for _, adapter := range cfg.AdaptersOfType(config.AdapterTypeQQOfficial) {
		if adapter.QQOfficial != nil {
			return adapter, *adapter.QQOfficial, true
		}
	}
	return config.AdapterInstance{}, config.QQOfficialConfig{}, false
}
