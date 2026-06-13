package runtime

type ActionSegment struct {
	Type string
	Data map[string]any
}

type Action struct {
	Kind                    string
	RawData                 map[string]any
	TargetType              string
	TargetID                string
	ReplyToEventID          string
	FallbackToSendIfMissing bool
	MessageSegments         []ActionSegment
	LogLevel                string
	LogMessage              string
	LogFields               map[string]any
	ConfigKeys              []string
	PluginListVisibility    string
	SecretKey               string
	ConfigValues            map[string]any
	GovernanceOperation     string
	GovernanceEntryType     string
	GovernanceTargetID      string
	GovernanceReason        string
	GovernanceEnabled       *bool
	StorageOperation        string
	StorageRoot             string
	StoragePath             string
	StorageKey              string
	StoragePrefix           string
	StorageValue            any
	StorageContent          []byte
	HTTPMethod              string
	HTTPURL                 string
	HTTPHeaders             map[string]string
	HTTPTimeoutSeconds      int
	HTTPBody                []byte
	SchedulerTaskID         string
	SchedulerLogLabel       string
	SchedulerCron           string
	SchedulerEventType      string
	SchedulerPayload        map[string]any
	WebhookRoute            string
	WebhookMethods          []string
	WebhookAuthStrategy     string
	WebhookHeader           string
	WebhookSecretRef        string
	WebhookSignaturePrefix  string
	WebhookSourceIPs        []string
	WebhookReplayProtection *WebhookReplayProtection
	RenderTemplate          string
	RenderTheme             string
	RenderOutput            string
	RenderFallbackText      string
	RenderData              map[string]any
}

// WebhookReplayProtection mirrors the formal replay_protection contract on
// event.expose_webhook actions. TimestampHeader and EventIDHeader name the
// HTTP headers carrying the client-side replay nonce; ToleranceSeconds is
// the maximum acceptable skew against the server clock. Enforce=false
// degrades all replay rejections to log-only observation while still
// counting them in metrics.
type WebhookReplayProtection struct {
	TimestampHeader  string
	EventIDHeader    string
	ToleranceSeconds int
	Enforce          bool
}
