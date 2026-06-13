package runtime

import "time"

type BotInfo struct {
	ID       string
	Nickname string
}

type InitPayload struct {
	Bot             BotInfo
	Capabilities    []string
	SuperAdmins     []string
	CommandPrefixes []string
}

type Spec struct {
	PluginID             string
	Runtime              string
	Command              string
	Args                 []string
	Env                  []string
	WorkDir              string
	EntryPath            string
	InitTimeout          time.Duration
	InitMaxTotal         time.Duration
	EventTimeout         time.Duration
	ShutdownGrace        time.Duration
	EffectiveConcurrency int
}
