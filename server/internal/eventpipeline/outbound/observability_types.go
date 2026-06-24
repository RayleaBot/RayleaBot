package outbound

import (
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
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
