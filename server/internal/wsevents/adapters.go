package wsevents

import (
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
)

// AdapterIdentity is the bot a connected adapter authenticates as.
type AdapterIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AdapterDescriptor describes one chat adapter for the management surface,
// including adapters that are not configured yet so the surface can offer them
// as something to add.
type AdapterDescriptor struct {
	Protocol    string           `json:"protocol"`
	DisplayName string           `json:"display_name"`
	Configured  bool             `json:"configured"`
	Enabled     bool             `json:"enabled"`
	State       string           `json:"state"`
	Summary     string           `json:"summary"`
	Identity    *AdapterIdentity `json:"identity,omitempty"`
}

// QQOfficialStatusSource is the slice of the QQ client this package needs, so
// the service does not depend on the concrete adapter for a status read.
type QQOfficialStatusSource interface {
	Status() qqofficial.Status
}

// Adapters lists every formally supported chat adapter. An adapter that is not
// configured still appears, reported as unconfigured and idle.
func (s *ProtocolService) Adapters() []AdapterDescriptor {
	return []AdapterDescriptor{
		s.oneBot11Descriptor(),
		s.qqOfficialDescriptor(),
	}
}

func (s *ProtocolService) oneBot11Descriptor() AdapterDescriptor {
	descriptor := AdapterDescriptor{
		Protocol:    "onebot11",
		DisplayName: "OneBot11",
		State:       string(onebot11.StateStopped),
		Summary:     "适配器未启动。",
	}
	if s.adapter == nil {
		return descriptor
	}
	snapshot := s.adapter.Snapshot()
	descriptor.State = string(snapshot.State)
	// OneBot spreads its state across four transports: it counts as configured
	// once any one of them is, and as enabled once any one is turned on.
	for _, transport := range []onebot11.TransportSnapshot{
		snapshot.ReverseWS, snapshot.ForwardWS, snapshot.HTTPAPI, snapshot.Webhook,
	} {
		if transport.Configured {
			descriptor.Configured = true
		}
		if transport.Enabled {
			descriptor.Enabled = true
		}
	}
	descriptor.Summary = oneBot11Summary(descriptor, snapshot)
	if snapshot.BotID != "" {
		descriptor.Identity = &AdapterIdentity{ID: snapshot.BotID}
	}
	return descriptor
}

func (s *ProtocolService) qqOfficialDescriptor() AdapterDescriptor {
	descriptor := AdapterDescriptor{
		Protocol:    "qqofficial",
		DisplayName: "QQ 官方机器人",
		State:       qqofficial.StateIdle,
		Summary:     "适配器未配置。",
	}
	cfg := s.config.CurrentConfig().QQOfficial
	descriptor.Configured = cfg.AppID != "" && cfg.AppSecret != ""
	descriptor.Enabled = cfg.Enabled
	if s.qqOfficial == nil {
		if descriptor.Configured && !descriptor.Enabled {
			descriptor.Summary = "适配器已配置但未启用。"
		}
		return descriptor
	}
	status := s.qqOfficial.Status()
	descriptor.State = status.State
	descriptor.Summary = status.Summary
	if status.BotID != "" {
		descriptor.Identity = &AdapterIdentity{ID: status.BotID, Name: status.BotName}
	}
	return descriptor
}

func oneBot11Summary(descriptor AdapterDescriptor, snapshot onebot11.Snapshot) string {
	switch {
	case !descriptor.Configured:
		return "适配器未配置。"
	case !descriptor.Enabled:
		return "适配器已配置但未启用。"
	case snapshot.State == onebot11.StateConnected:
		return "已连接。"
	case snapshot.LastErrorMessage != "":
		return snapshot.LastErrorMessage
	default:
		return "等待连接。"
	}
}
