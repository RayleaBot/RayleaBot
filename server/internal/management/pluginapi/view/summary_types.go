package view

type SummaryResponse struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Version           string              `json:"version,omitempty"`
	Description       string              `json:"description,omitempty"`
	Author            string              `json:"author,omitempty"`
	Role              string              `json:"role"`
	RegistrationState string              `json:"registration_state"`
	DesiredState      string              `json:"desired_state"`
	RuntimeState      string              `json:"runtime_state"`
	DisplayState      string              `json:"display_state"`
	Source            SourceResponse      `json:"source"`
	Trust             TrustResponse       `json:"trust"`
	Commands          []CommandResponse   `json:"commands"`
	Help              HelpResponse        `json:"help"`
	CommandConflicts  []string            `json:"command_conflicts"`
	DeadLetter        *DeadLetterResponse `json:"dead_letter,omitempty"`
}

type CommandResponse struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases,omitempty"`
	Description   string   `json:"description,omitempty"`
	Usage         string   `json:"usage,omitempty"`
	Permission    string   `json:"permission,omitempty"`
	CommandSource string   `json:"command_source"`
	DeclarationID string   `json:"declaration_id,omitempty"`
}

type SourceResponse struct {
	Root              string `json:"root"`
	PackageSourceType string `json:"package_source_type,omitempty"`
	PackageSourceRef  string `json:"package_source_ref,omitempty"`
	Verified          bool   `json:"verified"`
}

type TrustResponse struct {
	Level string `json:"level"`
	Label string `json:"label"`
}

type HelpResponse struct {
	Title   string              `json:"title,omitempty"`
	Summary string              `json:"summary,omitempty"`
	Groups  []HelpGroupResponse `json:"groups"`
}

type HelpGroupResponse struct {
	Title string             `json:"title"`
	Items []HelpItemResponse `json:"items"`
}

type HelpItemResponse struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Usage       string `json:"usage,omitempty"`
	Command     string `json:"command,omitempty"`
	Permission  string `json:"permission,omitempty"`
}

type ListResponse struct {
	Items []SummaryResponse `json:"items"`
}
