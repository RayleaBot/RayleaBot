package managementhttp

import (
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
)

func (s *ProtocolService) currentOneBot11ProtocolSnapshot() oneBot11ProtocolSnapshotView {
	adapterSnapshot := adaptershell.Snapshot{}
	if s.adapter != nil {
		adapterSnapshot = s.adapter.Snapshot()
	}

	transports := []struct {
		key      adaptershell.TransportKey
		snapshot adaptershell.TransportSnapshot
	}{
		{key: adaptershell.TransportReverseWS, snapshot: adapterSnapshot.ReverseWS},
		{key: adaptershell.TransportForwardWS, snapshot: adapterSnapshot.ForwardWS},
		{key: adaptershell.TransportHTTPAPI, snapshot: adapterSnapshot.HTTPAPI},
		{key: adaptershell.TransportWebhook, snapshot: adapterSnapshot.Webhook},
	}

	configured := make([]string, 0, len(transports))
	status := make([]protocolTransportStatusResponse, 0, len(transports))
	for _, transport := range transports {
		if transport.snapshot.Configured {
			configured = append(configured, string(transport.key))
		}
		runtimeInfo := transport.snapshot.RuntimeInfo
		status = append(status, protocolTransportStatusResponse{
			Transport:       string(transport.key),
			Enabled:         transport.snapshot.Enabled,
			Configured:      transport.snapshot.Configured,
			Endpoint:        transport.snapshot.Endpoint,
			State:           string(transport.snapshot.State),
			Summary:         protocolTransportSummary(transport.key, transport.snapshot),
			Provider:        currentOneBotProvider(runtimeInfo.Provider),
			AppName:         runtimeInfo.AppName,
			ProtocolVersion: runtimeInfo.ProtocolVersion,
			AppVersion:      runtimeInfo.AppVersion,
			UserID:          runtimeInfo.UserID,
			Nickname:        runtimeInfo.Nickname,
		})
	}

	active := make([]string, 0, len(adapterSnapshot.ActiveTransports))
	for _, key := range adapterSnapshot.ActiveTransports {
		active = append(active, string(key))
	}

	readiness := protocolReadinessStatus(adapterSnapshot)
	return oneBot11ProtocolSnapshotView{
		Protocol:              "onebot11",
		Provider:              adapterSnapshot.DetectedProvider(),
		ConfiguredTransports:  configured,
		ActiveTransports:      active,
		TransportStatus:       status,
		ReadinessStatus:       readiness,
		Summary:               protocolSnapshotSummary(adapterSnapshot, readiness),
		RecentTransportIssues: protocolIssuesFromSnapshot(adapterSnapshot),
	}
}

func (s *ProtocolService) transportIngressEnabled(transport adaptershell.TransportKey) bool {
	if s == nil || s.adapter == nil {
		return false
	}

	snapshot := s.adapter.Snapshot()
	switch transport {
	case adaptershell.TransportReverseWS:
		return snapshot.ReverseWS.Enabled && snapshot.ReverseWS.Configured
	case adaptershell.TransportWebhook:
		return snapshot.Webhook.Enabled && snapshot.Webhook.Configured
	default:
		return false
	}
}
