package managementhttp

import "github.com/RayleaBot/RayleaBot/server/internal/adapter"

func (s *ProtocolService) currentOneBot11ProtocolSnapshot() oneBot11ProtocolSnapshotResponse {
	adapterSnapshot := adapter.Snapshot{}
	if s.adapter != nil {
		adapterSnapshot = s.adapter.Snapshot()
	}

	transports := []struct {
		key      adapter.TransportKey
		snapshot adapter.TransportSnapshot
	}{
		{key: adapter.TransportReverseWS, snapshot: adapterSnapshot.ReverseWS},
		{key: adapter.TransportForwardWS, snapshot: adapterSnapshot.ForwardWS},
		{key: adapter.TransportHTTPAPI, snapshot: adapterSnapshot.HTTPAPI},
		{key: adapter.TransportWebhook, snapshot: adapterSnapshot.Webhook},
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
	return oneBot11ProtocolSnapshotResponse{
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

func (s *ProtocolService) transportIngressEnabled(transport adapter.TransportKey) bool {
	if s == nil || s.adapter == nil {
		return false
	}

	snapshot := s.adapter.Snapshot()
	switch transport {
	case adapter.TransportReverseWS:
		return snapshot.ReverseWS.Enabled && snapshot.ReverseWS.Configured
	case adapter.TransportWebhook:
		return snapshot.Webhook.Enabled && snapshot.Webhook.Configured
	default:
		return false
	}
}
