package plugins

import (
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
)

// Action contains decoded host capability arguments. Message delivery receives
// only MessageCommand; process framing remains in the runtime package.
type Action struct {
	Kind                         string
	RawData                      map[string]any
	SourceProtocol               string
	SourceAdapter                string
	TargetType                   string
	TargetID                     string
	ReplyToEventID               string
	FallbackToSendIfMissing      bool
	MessageSegments              []chatevent.MessageSegment
	LogLevel                     string
	LogMessage                   string
	LogFields                    map[string]any
	PluginListVisibility         string
	SecretKey                    string
	ThirdPartyAccountPlatform    string
	ThirdPartyAccountID          string
	ThirdPartyAccountObservation string
	ThirdPartyAccountHTTPStatus  int
	ThirdPartyResolveQuery       string
	ThirdPartyResolveCookie      string
	ConfigValues                 map[string]any
	GovernanceScope              chatevent.IdentityScope
	GovernanceOperation          string
	GovernanceEntryType          string
	GovernanceTargetID           string
	GovernanceReason             string
	GovernanceEnabled            *bool
	StorageOperation             string
	StoragePath                  string
	StorageKey                   string
	StoragePrefix                string
	StorageValue                 any
	StorageContent               []byte
	HTTPMethod                   string
	HTTPURL                      string
	HTTPHeaders                  map[string]string
	HTTPTimeoutSeconds           int
	HTTPBody                     []byte
	SchedulerTaskID              string
	SchedulerLogLabel            string
	SchedulerCron                string
	SchedulerEventType           string
	SchedulerPayload             map[string]any
	RenderTemplate               string
	RenderTheme                  string
	RenderOutput                 string
	RenderFallbackText           string
	RenderData                   map[string]any
	RenderResources              []RenderImageResource
}

func (a Action) MessageCommand() chatevent.MessageCommand {
	return chatevent.MessageCommand{
		Kind:                    a.Kind,
		SourceProtocol:          a.SourceProtocol,
		SourceAdapter:           a.SourceAdapter,
		TargetType:              a.TargetType,
		TargetID:                a.TargetID,
		ReplyToEventID:          a.ReplyToEventID,
		FallbackToSendIfMissing: a.FallbackToSendIfMissing,
		MessageSegments:         a.MessageSegments,
	}
}

type RenderImageResource struct {
	ID           string
	URL          string
	FallbackURLs []string
	Referer      string
}
