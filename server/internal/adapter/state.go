package adapter

import "time"

type State string

const (
	StateIdle         State = "idle"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateAuthFailed   State = "auth_failed"
	StateReconnecting State = "reconnecting"
	StateStopped      State = "stopped"
)

type Snapshot struct {
	State                 State
	BotID                 string
	LastErrorCode         string
	LastErrorMessage      string
	ReadyFrameSeen        bool
	ConnectedAt           *time.Time
	LastFrameAt           *time.Time
	LastHeartbeatAt       *time.Time
	HeartbeatInterval     time.Duration
	TotalReceivedFrames   uint64
	InvalidReceivedFrames uint64
	HeartbeatSeen         bool
	LastFrameCategory     FrameCategory
	LastFrameType         string
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.ConnectedAt = cloneTime(snapshot.ConnectedAt)
	cloned.LastFrameAt = cloneTime(snapshot.LastFrameAt)
	cloned.LastHeartbeatAt = cloneTime(snapshot.LastHeartbeatAt)
	return cloned
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	copied := *value
	return &copied
}
