package outbound

import (
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

type SendAttempt struct {
	ActionKind string
	TargetType string
	TargetID   string
	Segments   []onebot11.OutboundMessageSegment
}

type SendLogContext struct {
	PluginID    string
	RequestID   string
	CommandName string
	TargetLabel string
}
