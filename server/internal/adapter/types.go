package adapter

import (
	"time"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
)

type Error = adaptershell.Error

type Backoff = adaptershell.Backoff

func NewBackoff(initialSeconds int, multiplier float64, maxSeconds int, jitterRatio float64, randFloat func() float64) *Backoff {
	return adaptershell.NewBackoff(initialSeconds, multiplier, maxSeconds, jitterRatio, randFloat)
}

type IdentityCache = adaptershell.IdentityCache

func NewIdentityCache(ttl time.Duration) *IdentityCache {
	return adaptershell.NewIdentityCache(ttl)
}

type FrameCategory = adaptershell.FrameCategory

const (
	FrameCategoryLifecycleReady = adaptershell.FrameCategoryLifecycleReady
	FrameCategoryHeartbeat      = adaptershell.FrameCategoryHeartbeat
	FrameCategoryEvent          = adaptershell.FrameCategoryEvent
	FrameCategoryAPIResponse    = adaptershell.FrameCategoryAPIResponse
	FrameCategoryUnknown        = adaptershell.FrameCategoryUnknown
	FrameCategoryInvalid        = adaptershell.FrameCategoryInvalid
)

type FrameSummary = adaptershell.FrameSummary

const (
	EventKindMessageText = adaptershell.EventKindMessageText
	EventKindMessage     = adaptershell.EventKindMessage
	EventKindMessageSent = adaptershell.EventKindMessageSent
	EventKindNotice      = adaptershell.EventKindNotice
	EventKindRequest     = adaptershell.EventKindRequest
	EventKindMeta        = adaptershell.EventKindMeta
)

type NormalizedEvent = adaptershell.NormalizedEvent
type MessageSegment = adaptershell.MessageSegment

type LoginInfo = adaptershell.LoginInfo
type VersionInfo = adaptershell.VersionInfo
type GroupMemberInfo = adaptershell.GroupMemberInfo
type GroupInfo = adaptershell.GroupInfo
type GroupTarget = adaptershell.GroupTarget
type FriendTarget = adaptershell.FriendTarget
type StrangerInfo = adaptershell.StrangerInfo

type OutboundMessageSend = adaptershell.OutboundMessageSend
type OutboundMessageReply = adaptershell.OutboundMessageReply
type OutboundMessageSegment = adaptershell.OutboundMessageSegment
type SendMessageResult = adaptershell.SendMessageResult

func OutboundSegmentsToPlainText(segments []OutboundMessageSegment) string {
	return adaptershell.OutboundSegmentsToPlainText(segments)
}
