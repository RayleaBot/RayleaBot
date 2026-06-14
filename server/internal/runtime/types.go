package runtime

import runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"

type State = runtimemanager.State

const (
	StateStopped    = runtimemanager.StateStopped
	StateStarting   = runtimemanager.StateStarting
	StateRunning    = runtimemanager.StateRunning
	StateStopping   = runtimemanager.StateStopping
	StateCrashed    = runtimemanager.StateCrashed
	StateBackoff    = runtimemanager.StateBackoff
	StateDeadLetter = runtimemanager.StateDeadLetter
)

type Snapshot = runtimemanager.Snapshot
type CrashCallback = runtimemanager.CrashCallback
type LocalActionExecutor = runtimemanager.LocalActionExecutor
type Options = runtimemanager.Options

type ActionSegment = runtimemanager.ActionSegment
type Action = runtimemanager.Action
type WebhookReplayProtection = runtimemanager.WebhookReplayProtection

type Event = runtimemanager.Event
type SchedulerLogContext = runtimemanager.SchedulerLogContext
type SchedulerRunRecorder = runtimemanager.SchedulerRunRecorder
type SchedulerRunResult = runtimemanager.SchedulerRunResult
type EventActor = runtimemanager.EventActor
type EventTarget = runtimemanager.EventTarget
type EventMessage = runtimemanager.EventMessage
type EventSegment = runtimemanager.EventSegment

type Delivery = runtimemanager.Delivery
type BotInfo = runtimemanager.BotInfo
type InitPayload = runtimemanager.InitPayload
type Spec = runtimemanager.Spec
