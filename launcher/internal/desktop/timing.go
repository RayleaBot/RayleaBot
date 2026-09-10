package desktop

import "time"

// Each budget belongs to one operation; equal durations do not imply a shared
// lifecycle or cancellation policy.
const (
	managementRequestTimeout    = 5 * time.Second
	refreshRequestBudget        = 6 * time.Second
	startHealthProbeBudget      = 6 * time.Second
	startupReadinessBudget      = 15 * time.Minute
	startupProbeTimeout         = 3 * time.Second
	startupFailureStabilization = 10 * time.Second
	startupPollInterval         = 500 * time.Millisecond
	shutdownRequestTimeout      = 6 * time.Second
	resetAdminReadinessBudget   = 30 * time.Second
	resetAdminProbeTimeout      = 3 * time.Second
	quickHealthProbeTimeout     = 2 * time.Second
	endpointConnectTimeout      = 400 * time.Millisecond
	offlineOperationTimeout     = 15 * time.Second
)
