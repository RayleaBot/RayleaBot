package outbound

import "github.com/RayleaBot/RayleaBot/server/internal/adapter"

type SendAttempt struct {
	ActionKind string
	TargetType string
	TargetID   string
	Segments   []adapter.OutboundMessageSegment
}

type SendLogContext struct {
	PluginID    string
	RequestID   string
	CommandName string
	TargetLabel string
}
