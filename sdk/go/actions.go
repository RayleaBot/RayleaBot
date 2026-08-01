package rayleabot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ActionResult map[string]any

type Actions struct {
	event *EventContext
}

func (actions *Actions) Call(ctx context.Context, action string, input any, output any) error {
	if actions == nil || actions.event == nil || actions.event.client == nil {
		return errors.New("rayleabot: action client is unavailable")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return errors.New("rayleabot: action name is required")
	}
	client := actions.event.client
	requestID := client.nextRequestID(actions.event.RequestID)
	response := make(chan protocolFrame, 1)
	client.pendingMu.Lock()
	client.pending[requestID] = response
	client.pendingMu.Unlock()

	data, err := json.Marshal(input)
	if err != nil {
		client.removePending(requestID)
		return fmt.Errorf("rayleabot: marshal %s action: %w", action, err)
	}
	if err := client.writer.write(protocolFrame{
		ProtocolVersion: ProtocolVersion,
		Type:            "action",
		Timestamp:       time.Now().Unix(),
		PluginID:        actions.event.PluginID,
		RequestID:       requestID,
		ParentRequestID: actions.event.RequestID,
		Action:          action,
		Data:            data,
	}); err != nil {
		client.removePending(requestID)
		return err
	}

	waitCtx := ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	if _, ok := waitCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(waitCtx, client.actionTimeout)
		defer cancel()
	}
	select {
	case frame := <-response:
		if frame.Type == "error" {
			code := frame.Code
			if code == "" {
				code = "plugin.internal_error"
			}
			message := frame.Message
			if message == "" {
				message = "local action failed"
			}
			return &ActionError{Code: code, Message: message, Details: frame.Details}
		}
		if output == nil || len(frame.Data) == 0 || string(frame.Data) == "null" {
			return nil
		}
		if err := json.Unmarshal(frame.Data, output); err != nil {
			return fmt.Errorf("rayleabot: decode %s action response: %w", action, err)
		}
		return nil
	case <-waitCtx.Done():
		client.removePending(requestID)
		return fmt.Errorf("rayleabot: %s action: %w", action, waitCtx.Err())
	}
}

func (client *runtimeClient) removePending(requestID string) {
	client.pendingMu.Lock()
	delete(client.pending, requestID)
	client.pendingMu.Unlock()
}

func (actions *Actions) callResult(ctx context.Context, action string, input any) (ActionResult, error) {
	var result ActionResult
	if err := actions.Call(ctx, action, input, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = ActionResult{}
	}
	return result, nil
}

type MessageSendRequest struct {
	TargetType string     `json:"target_type"`
	TargetID   string     `json:"target_id"`
	Message    MessageOut `json:"message"`
}

type MessageOut struct {
	Segments []Segment `json:"segments"`
}

func (actions *Actions) MessageSend(ctx context.Context, request MessageSendRequest) (ActionResult, error) {
	return actions.callResult(ctx, "message.send", request)
}

type LoggerWriteRequest struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (actions *Actions) LoggerWrite(ctx context.Context, request LoggerWriteRequest) (ActionResult, error) {
	return actions.callResult(ctx, "logger.write", request)
}

type KVRequest struct {
	Operation string `json:"operation"`
	Key       string `json:"key,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Value     any    `json:"value,omitempty"`
}

func (actions *Actions) KVGet(ctx context.Context, key string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.kv", KVRequest{Operation: "get", Key: key})
}

func (actions *Actions) KVSet(ctx context.Context, key string, value any) (ActionResult, error) {
	return actions.callResult(ctx, "storage.kv", KVRequest{Operation: "set", Key: key, Value: value})
}

func (actions *Actions) KVDelete(ctx context.Context, key string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.kv", KVRequest{Operation: "delete", Key: key})
}

func (actions *Actions) KVList(ctx context.Context, prefix string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.kv", KVRequest{Operation: "list", Prefix: prefix})
}

type FileRequest struct {
	Operation     string `json:"operation"`
	Root          string `json:"root"`
	Path          string `json:"path,omitempty"`
	Prefix        string `json:"prefix,omitempty"`
	ContentText   string `json:"content_text,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

func (actions *Actions) FileRead(ctx context.Context, path string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "read", Root: "plugin_data", Path: path})
}

func (actions *Actions) FileWriteText(ctx context.Context, path, content string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "write", Root: "plugin_data", Path: path, ContentText: content})
}

func (actions *Actions) FileWriteBase64(ctx context.Context, path, content string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "write", Root: "plugin_data", Path: path, ContentBase64: content})
}

func (actions *Actions) FileDelete(ctx context.Context, path string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "delete", Root: "plugin_data", Path: path})
}

func (actions *Actions) FileList(ctx context.Context, prefix string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "list", Root: "plugin_data", Prefix: prefix})
}

type HTTPRequest struct {
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	BodyText       string            `json:"body_text,omitempty"`
	BodyBase64     string            `json:"body_base64,omitempty"`
}

func (actions *Actions) HTTPRequest(ctx context.Context, request HTTPRequest) (ActionResult, error) {
	if request.BodyText != "" && request.BodyBase64 != "" {
		return nil, errors.New("rayleabot: HTTPRequest accepts at most one body representation")
	}
	return actions.callResult(ctx, "http.request", request)
}

type ConfigReadRequest struct {
	Keys []string `json:"keys"`
}

type ConfigWriteRequest struct {
	Values map[string]any `json:"values"`
}

func (actions *Actions) ConfigRead(ctx context.Context, keys ...string) (ActionResult, error) {
	if len(keys) == 0 {
		return nil, errors.New("rayleabot: ConfigRead requires at least one key")
	}
	return actions.callResult(ctx, "config.read", ConfigReadRequest{Keys: keys})
}

func (actions *Actions) ConfigWrite(ctx context.Context, values map[string]any) (ActionResult, error) {
	if len(values) == 0 {
		return nil, errors.New("rayleabot: ConfigWrite requires at least one value")
	}
	return actions.callResult(ctx, "config.write", ConfigWriteRequest{Values: values})
}

type PluginVisibility string

const (
	PluginVisibilityCatalog PluginVisibility = "catalog"
	PluginVisibilityCaller  PluginVisibility = "caller"
)

func (actions *Actions) PluginList(ctx context.Context, visibility ...PluginVisibility) (ActionResult, error) {
	request := struct {
		Visibility PluginVisibility `json:"visibility,omitempty"`
	}{}
	if len(visibility) > 0 {
		request.Visibility = visibility[0]
	}
	return actions.callResult(ctx, "plugin.list", request)
}

type SecretReadRequest struct {
	Key string `json:"key"`
}

func (actions *Actions) SecretRead(ctx context.Context, key string) (ActionResult, error) {
	return actions.callResult(ctx, "secret.read", SecretReadRequest{Key: key})
}

type ThirdPartyAccountReadRequest struct {
	Platform  string `json:"platform"`
	AccountID string `json:"account_id,omitempty"`
}

func (actions *Actions) ThirdPartyAccountRead(ctx context.Context, request ThirdPartyAccountReadRequest) (ActionResult, error) {
	return actions.callResult(ctx, "thirdparty.account.read", request)
}

func (actions *Actions) GovernanceBlacklistRead(ctx context.Context) (ActionResult, error) {
	return actions.callResult(ctx, "governance.blacklist.read", struct{}{})
}

type GovernanceBlacklistWriteRequest struct {
	Operation string `json:"operation"`
	EntryType string `json:"entry_type"`
	TargetID  string `json:"target_id"`
	Reason    string `json:"reason,omitempty"`
}

func (actions *Actions) GovernanceBlacklistWrite(ctx context.Context, request GovernanceBlacklistWriteRequest) (ActionResult, error) {
	return actions.callResult(ctx, "governance.blacklist.write", request)
}

func (actions *Actions) GovernanceWhitelistRead(ctx context.Context) (ActionResult, error) {
	return actions.callResult(ctx, "governance.whitelist.read", struct{}{})
}

type GovernanceWhitelistWriteRequest struct {
	Operation string `json:"operation"`
	Enabled   *bool  `json:"enabled,omitempty"`
	EntryType string `json:"entry_type,omitempty"`
	TargetID  string `json:"target_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func (actions *Actions) GovernanceWhitelistWrite(ctx context.Context, request GovernanceWhitelistWriteRequest) (ActionResult, error) {
	return actions.callResult(ctx, "governance.whitelist.write", request)
}

func (actions *Actions) GovernanceCommandPolicyRead(ctx context.Context) (ActionResult, error) {
	return actions.callResult(ctx, "governance.command_policy.read", struct{}{})
}

type SchedulerCreateRequest struct {
	TaskID    string         `json:"task_id"`
	Cron      string         `json:"cron"`
	EventType string         `json:"event_type"`
	LogLabel  string         `json:"log_label,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

func (actions *Actions) SchedulerCreate(ctx context.Context, request SchedulerCreateRequest) (ActionResult, error) {
	if request.EventType == "" {
		request.EventType = "scheduler.trigger"
	}
	return actions.callResult(ctx, "scheduler.create", request)
}

type ReplayProtection struct {
	TimestampHeader  string `json:"timestamp_header"`
	EventIDHeader    string `json:"event_id_header"`
	ToleranceSeconds int    `json:"tolerance_seconds"`
	Enforce          bool   `json:"enforce"`
}

type ExposeWebhookRequest struct {
	Route            string           `json:"route"`
	Methods          []string         `json:"methods"`
	AuthStrategy     string           `json:"auth_strategy"`
	Header           string           `json:"header"`
	SecretRef        string           `json:"secret_ref"`
	SignaturePrefix  string           `json:"signature_prefix,omitempty"`
	SourceIPs        []string         `json:"source_ips,omitempty"`
	ReplayProtection ReplayProtection `json:"replay_protection"`
}

func (actions *Actions) ExposeWebhook(ctx context.Context, request ExposeWebhookRequest) (ActionResult, error) {
	if request.SecretRef == "" {
		return nil, errors.New("rayleabot: ExposeWebhook requires SecretRef")
	}
	if len(request.Methods) == 0 {
		request.Methods = []string{"POST"}
	}
	if request.AuthStrategy == "" {
		request.AuthStrategy = "fixed_token"
	}
	if request.Header == "" {
		request.Header = "X-Webhook-Token"
	}
	if request.ReplayProtection.TimestampHeader == "" {
		request.ReplayProtection.TimestampHeader = "X-Raylea-Timestamp"
	}
	if request.ReplayProtection.EventIDHeader == "" {
		request.ReplayProtection.EventIDHeader = "X-Raylea-Event-Id"
	}
	if request.ReplayProtection.ToleranceSeconds == 0 {
		request.ReplayProtection.ToleranceSeconds = 300
	}
	return actions.callResult(ctx, "event.expose_webhook", request)
}

type RenderImageRequest struct {
	Template     string         `json:"template"`
	Data         map[string]any `json:"data"`
	Theme        string         `json:"theme,omitempty"`
	Output       string         `json:"output,omitempty"`
	FallbackText string         `json:"fallback_text,omitempty"`
}

func (actions *Actions) RenderImage(ctx context.Context, request RenderImageRequest) (ActionResult, error) {
	return actions.callResult(ctx, "render.image", request)
}

type OneBotAction string

const (
	ActionMessageGet              OneBotAction = "message.get"
	ActionMessageDelete           OneBotAction = "message.delete"
	ActionMessageHistoryGet       OneBotAction = "message.history.get"
	ActionMessageForwardGet       OneBotAction = "message.forward.get"
	ActionMessageForwardSend      OneBotAction = "message.forward.send"
	ActionMessageReadMark         OneBotAction = "message.read.mark"
	ActionFriendRequestHandle     OneBotAction = "friend.request.handle"
	ActionFriendList              OneBotAction = "friend.list"
	ActionFriendRemarkSet         OneBotAction = "friend.remark.set"
	ActionUserInfoGet             OneBotAction = "user.info.get"
	ActionUserLikeSend            OneBotAction = "user.like.send"
	ActionGroupList               OneBotAction = "group.list"
	ActionGroupInfoGet            OneBotAction = "group.info.get"
	ActionGroupMemberGet          OneBotAction = "group.member.get"
	ActionGroupMemberList         OneBotAction = "group.member.list"
	ActionGroupRequestHandle      OneBotAction = "group.request.handle"
	ActionGroupLeave              OneBotAction = "group.leave"
	ActionGroupAdminSet           OneBotAction = "group.admin.set"
	ActionGroupBanSet             OneBotAction = "group.ban.set"
	ActionGroupCardSet            OneBotAction = "group.card.set"
	ActionGroupTitleSet           OneBotAction = "group.title.set"
	ActionGroupNameSet            OneBotAction = "group.name.set"
	ActionGroupAnnouncementList   OneBotAction = "group.announcement.list"
	ActionGroupAnnouncementCreate OneBotAction = "group.announcement.create"
	ActionGroupAnnouncementDelete OneBotAction = "group.announcement.delete"
	ActionGroupEssenceList        OneBotAction = "group.essence.list"
	ActionGroupEssenceSet         OneBotAction = "group.essence.set"
	ActionGroupEssenceUnset       OneBotAction = "group.essence.unset"
	ActionGroupHonorGet           OneBotAction = "group.honor.get"
	ActionGroupTodoSet            OneBotAction = "group.todo.set"
	ActionFileGet                 OneBotAction = "file.get"
	ActionFileDownload            OneBotAction = "file.download"
	ActionFileGroupUpload         OneBotAction = "file.group.upload"
	ActionFilePrivateUpload       OneBotAction = "file.private.upload"
	ActionFileGroupURLGet         OneBotAction = "file.group.url.get"
	ActionFilePrivateURLGet       OneBotAction = "file.private.url.get"
	ActionFileGroupFSInfo         OneBotAction = "file.group.fs.info"
	ActionFileGroupFSList         OneBotAction = "file.group.fs.list"
	ActionFileGroupFSMkdir        OneBotAction = "file.group.fs.mkdir"
	ActionFileGroupFSDelete       OneBotAction = "file.group.fs.delete"
	ActionReactionSet             OneBotAction = "reaction.set"
	ActionReactionList            OneBotAction = "reaction.list"
	ActionPokeSend                OneBotAction = "poke.send"
)

func (actions *Actions) OneBot(ctx context.Context, action OneBotAction, request any, response any) error {
	return actions.Call(ctx, string(action), request, response)
}

type ProviderAction string

const (
	ActionNapCatMessageEmojiLikeSet  ProviderAction = "provider.napcat.message_emoji.like.set"
	ActionNapCatGroupSignSet         ProviderAction = "provider.napcat.group.sign.set"
	ActionLuckyLilliaFriendGroupsGet ProviderAction = "provider.luckylillia.friend_groups.get"
)

func (actions *Actions) Provider(ctx context.Context, action ProviderAction, request any, response any) error {
	return actions.Call(ctx, string(action), request, response)
}
