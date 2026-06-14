package bilibili

import (
	"time"

	bilibiliSource "github.com/RayleaBot/RayleaBot/server/internal/bilibili/source"
)

type Subscription = bilibiliSource.Subscription
type Subject = bilibiliSource.Subject
type BilibiliEvent = bilibiliSource.BilibiliEvent
type BilibiliOriginal = bilibiliSource.BilibiliOriginal
type BilibiliTopic = bilibiliSource.BilibiliTopic
type Author = bilibiliSource.Author
type Image = bilibiliSource.Image
type roomState = bilibiliSource.RoomState
type Status = bilibiliSource.Status
type Diagnosis = bilibiliSource.Diagnosis
type DiagnosisCause = bilibiliSource.DiagnosisCause
type DiagnosisAction = bilibiliSource.DiagnosisAction
type LiveStatus = bilibiliSource.LiveStatus
type DynamicStatus = bilibiliSource.DynamicStatus
type MonitorSnapshot = bilibiliSource.MonitorSnapshot
type MonitorItem = bilibiliSource.MonitorItem
type MonitorDynamic = bilibiliSource.MonitorDynamic
type MonitorLive = bilibiliSource.MonitorLive

const (
	StateDisabled   = bilibiliSource.StateDisabled
	StateIdle       = bilibiliSource.StateIdle
	StateConnecting = bilibiliSource.StateConnecting
	StateConnected  = bilibiliSource.StateConnected
	StateDegraded   = bilibiliSource.StateDegraded
	StateFailed     = bilibiliSource.StateFailed

	EventLiveStarted      = bilibiliSource.EventLiveStarted
	EventLiveEnded        = bilibiliSource.EventLiveEnded
	EventDynamicPublished = bilibiliSource.EventDynamicPublished
)

func DiagnosisForStatus(status Status, now time.Time) Diagnosis {
	return bilibiliSource.DiagnosisForStatus(status, now)
}
