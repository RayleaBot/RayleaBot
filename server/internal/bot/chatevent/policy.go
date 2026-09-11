package chatevent

type DeliveryOutcome string

const (
	DeliveryOutcomeIgnored   DeliveryOutcome = "ignored"
	DeliveryOutcomeDelivered DeliveryOutcome = "delivered"
	DeliveryOutcomeError     DeliveryOutcome = "error"
	DeliveryOutcomeRejected  DeliveryOutcome = "rejected"
)

type CommandPolicyRejection struct {
	CommandName      string
	PluginID         string
	MatchedPluginIDs []string
	ErrorCode        string
	Reason           string
	ReasonSummary    string
	PolicyStage      string
}
