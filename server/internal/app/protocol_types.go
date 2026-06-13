package app

type protocolIssueResponse struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

type protocolTransportStatusResponse struct {
	Transport       string `json:"transport"`
	Enabled         bool   `json:"enabled"`
	Configured      bool   `json:"configured"`
	Endpoint        string `json:"endpoint"`
	State           string `json:"state"`
	Summary         string `json:"summary"`
	Provider        string `json:"provider,omitempty"`
	AppName         string `json:"app_name,omitempty"`
	ProtocolVersion string `json:"protocol_version,omitempty"`
	AppVersion      string `json:"app_version,omitempty"`
	UserID          string `json:"user_id,omitempty"`
	Nickname        string `json:"nickname,omitempty"`
}

type oneBot11ProtocolSnapshotResponse = oneBot11ProtocolSnapshotView

type oneBot11ProtocolSnapshotView struct {
	Protocol              string                            `json:"protocol"`
	Provider              string                            `json:"provider"`
	ConfiguredTransports  []string                          `json:"configured_transports"`
	ActiveTransports      []string                          `json:"active_transports"`
	TransportStatus       []protocolTransportStatusResponse `json:"transport_status"`
	ReadinessStatus       string                            `json:"readiness_status"`
	Summary               string                            `json:"summary"`
	RecentTransportIssues []protocolIssueResponse           `json:"recent_transport_issues"`
}

type oneBot11TargetIssueResponse struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

type oneBot11GroupTargetResponse struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type oneBot11PrivateTargetResponse struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type oneBot11ProtocolTargetsResponse struct {
	Protocol     string                          `json:"protocol"`
	Available    bool                            `json:"available"`
	Groups       []oneBot11GroupTargetResponse   `json:"groups"`
	PrivateUsers []oneBot11PrivateTargetResponse `json:"private_users"`
	Issues       []oneBot11TargetIssueResponse   `json:"issues"`
}

type oneBot11IdentityResolveRequest struct {
	Items []oneBot11IdentityResolveItem `json:"items"`
}

type oneBot11IdentityResolveItem struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	UserID     string `json:"user_id"`
}

type oneBot11IdentityResponse struct {
	TargetType    string `json:"target_type"`
	TargetID      string `json:"target_id"`
	UserID        string `json:"user_id"`
	Nickname      string `json:"nickname"`
	GroupNickname string `json:"group_nickname,omitempty"`
	Title         string `json:"title,omitempty"`
	Role          string `json:"role,omitempty"`
	RoleLabel     string `json:"role_label,omitempty"`
	AvatarURL     string `json:"avatar_url"`
}

type oneBot11IdentityResolveResponse struct {
	Items  []oneBot11IdentityResponse    `json:"items"`
	Issues []oneBot11TargetIssueResponse `json:"issues"`
}

type protocolCompatibilitySupportResponse struct {
	Standard    string `json:"standard"`
	NapCat      string `json:"napcat"`
	LuckyLillia string `json:"luckylillia"`
}

type protocolCompatibilityItemResponse struct {
	Key     string                               `json:"key"`
	Label   string                               `json:"label"`
	Support protocolCompatibilitySupportResponse `json:"support"`
	Summary string                               `json:"summary"`
}

type protocolCompatibilityCategoryResponse struct {
	Key   string                              `json:"key"`
	Title string                              `json:"title"`
	Items []protocolCompatibilityItemResponse `json:"items"`
}

type oneBot11ProtocolCompatibilityResponse struct {
	Protocol   string                                  `json:"protocol"`
	Categories []protocolCompatibilityCategoryResponse `json:"categories"`
}
