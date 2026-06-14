package outbound

import (
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
)

type SendAttempt struct {
	ActionKind string
	TargetType string
	TargetID   string
	Segments   []adapteroutbound.OutboundMessageSegment
}

type SendLogContext struct {
	PluginID    string
	RequestID   string
	CommandName string
	TargetLabel string
}
