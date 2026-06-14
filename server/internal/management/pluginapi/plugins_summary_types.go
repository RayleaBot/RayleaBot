package pluginapi

type pluginSummaryResponse struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	Version           string                    `json:"version,omitempty"`
	Description       string                    `json:"description,omitempty"`
	Author            string                    `json:"author,omitempty"`
	Role              string                    `json:"role"`
	RegistrationState string                    `json:"registration_state"`
	DesiredState      string                    `json:"desired_state"`
	RuntimeState      string                    `json:"runtime_state"`
	DisplayState      string                    `json:"display_state"`
	Source            pluginSourceResponse      `json:"source"`
	Trust             pluginTrustResponse       `json:"trust"`
	Commands          []pluginCommandResponse   `json:"commands"`
	Help              pluginHelpResponse        `json:"help"`
	CommandConflicts  []string                  `json:"command_conflicts"`
	DeadLetter        *pluginDeadLetterResponse `json:"dead_letter,omitempty"`
}

type pluginDeadLetterResponse struct {
	EnteredAt        string `json:"entered_at"`
	CrashCount       int    `json:"crash_count"`
	LastErrorCode    string `json:"last_error_code,omitempty"`
	LastErrorMessage string `json:"last_error_message,omitempty"`
}

type pluginCommandResponse struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases,omitempty"`
	Description   string   `json:"description,omitempty"`
	Usage         string   `json:"usage,omitempty"`
	Permission    string   `json:"permission,omitempty"`
	CommandSource string   `json:"command_source"`
	DeclarationID string   `json:"declaration_id,omitempty"`
}

type pluginSourceResponse struct {
	Root              string `json:"root"`
	PackageSourceType string `json:"package_source_type,omitempty"`
	PackageSourceRef  string `json:"package_source_ref,omitempty"`
	Verified          bool   `json:"verified"`
}

type pluginTrustResponse struct {
	Level string `json:"level"`
	Label string `json:"label"`
}

type pluginHelpResponse struct {
	Title   string                    `json:"title,omitempty"`
	Summary string                    `json:"summary,omitempty"`
	Groups  []pluginHelpGroupResponse `json:"groups"`
}

type pluginHelpGroupResponse struct {
	Title string                   `json:"title"`
	Items []pluginHelpItemResponse `json:"items"`
}

type pluginHelpItemResponse struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Usage       string `json:"usage,omitempty"`
	Command     string `json:"command,omitempty"`
	Permission  string `json:"permission,omitempty"`
}

type pluginListResponse struct {
	Items []pluginSummaryResponse `json:"items"`
}
