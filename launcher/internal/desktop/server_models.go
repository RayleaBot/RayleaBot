package desktop

// Server responses read by the Launcher. The Server ships in the same package,
// so these structs keep only the fields the Launcher uses and ignore the rest.

type ServerAdapterProtocol string

type ServerAdapterState string

type ServerAdapterStatus struct {
	ID       string                `json:"id"`
	Protocol ServerAdapterProtocol `json:"protocol"`
	Enabled  bool                  `json:"enabled"`
	State    ServerAdapterState    `json:"state"`
}

type ServerDiagnosticIssue struct {
	Code             string   `json:"code"`
	Severity         string   `json:"severity"`
	Summary          string   `json:"summary"`
	UserMessage      string   `json:"user_message,omitempty"`
	Remediation      string   `json:"remediation,omitempty"`
	InternalReason   string   `json:"internal_reason,omitempty"`
	RuntimeResources []string `json:"runtime_resources,omitempty"`
}

type ServerLivenessStatusResponse struct {
	Status string `json:"status"`
}

type ServerReadinessStatusResponse struct {
	Status      string                               `json:"status"`
	Reason      string                               `json:"reason,omitempty"`
	ReasonCodes []string                             `json:"reason_codes,omitempty"`
	Checks      *ServerReadinessStatusResponseChecks `json:"checks,omitempty"`
	Issues      []ServerDiagnosticIssue              `json:"issues,omitempty"`
}

type ServerSystemShutdownResponse struct {
	Accepted bool `json:"accepted"`
}

type ServerSystemStatusResponse struct {
	ShutdownBudgetSeconds int64                          `json:"shutdown_budget_seconds,omitempty"`
	Status                string                         `json:"status"`
	Adapters              []ServerAdapterStatus          `json:"adapters"`
	ActivePlugins         int64                          `json:"active_plugins,omitempty"`
	RunningPlugins        int64                          `json:"running_plugins,omitempty"`
	FailedPlugins         int64                          `json:"failed_plugins,omitempty"`
	DBSchemaVersion       string                         `json:"db_schema_version,omitempty"`
	UptimeSeconds         int64                          `json:"uptime_seconds,omitempty"`
	Health                *ServerReadinessStatusResponse `json:"health,omitempty"`
}

type ServerReadinessStatusResponseChecks struct {
	Database string `json:"database,omitempty"`
	Runtime  string `json:"runtime,omitempty"`
	Render   string `json:"render,omitempty"`
}
