package rayleabot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("rayleabot: marshal %s action: %w", action, err)
	}
	client := actions.event.client
	requestID := client.nextRequestID(actions.event.RequestID)
	response := make(chan protocolFrame, 1)
	actions.event.actionMu.Lock()
	if actions.event.terminal.Load() {
		actions.event.actionMu.Unlock()
		return errors.New("rayleabot: event is closing; no further actions are allowed")
	}
	client.pendingMu.Lock()
	if client.closed || len(client.pending) >= pendingActionLimit {
		client.pendingMu.Unlock()
		actions.event.actionMu.Unlock()
		return errors.New("rayleabot: action client is closed or pending action limit reached")
	}
	if actions.event.actions == nil {
		actions.event.actions = make(map[string]chan struct{})
	}
	actions.event.actions[requestID] = make(chan struct{})
	client.pending[requestID] = &pendingAction{response: response, event: actions.event}
	client.pendingMu.Unlock()
	actions.event.actionMu.Unlock()
	if err := ctx.Err(); err != nil {
		client.removePending(requestID)
		return err
	}
	if err := client.writer.write(protocolFrame{
		Type:            "action",
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
		// The host may still execute the action. Keep its response association
		// until it settles or the event's bounded terminal drain retires it.
		return fmt.Errorf("rayleabot: %s action: %w", action, waitCtx.Err())
	}
}

func (client *runtimeClient) removePending(requestID string) {
	client.pendingMu.Lock()
	pending := client.pending[requestID]
	delete(client.pending, requestID)
	client.pendingMu.Unlock()
	if pending != nil {
		pending.event.finishAction(requestID)
	}
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
	// SourceProtocol names the chat adapter that must deliver the message.
	// A reply already knows it from the event it answers; an active push needs
	// it only while more than one adapter is connected, because target
	// identifiers are namespaced per protocol.
	SourceProtocol string     `json:"source_protocol,omitempty"`
	TargetType     string     `json:"target_type"`
	TargetID       string     `json:"target_id"`
	Message        MessageOut `json:"message"`
}

type MessageOut struct {
	Segments []Segment `json:"segments"`
}

func (actions *Actions) MessageSend(ctx context.Context, request MessageSendRequest) (ActionResult, error) {
	return actions.callResult(ctx, "message.send", request)
}

type LoggerWriteRequest struct {
	Level string `json:"level"`
	// Message is a redacted operator-facing narrative that remains understandable
	// without Fields. Include the known cause, impact, and recovery direction for
	// warnings and errors when that information is available.
	Message string `json:"message"`
	// Fields contains supplemental redacted diagnostics for filtering and detail
	// views; it must not carry context required to understand Message.
	Fields map[string]any `json:"fields,omitempty"`
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
	Path          string `json:"path,omitempty"`
	Prefix        string `json:"prefix,omitempty"`
	ContentText   string `json:"content_text,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

func (actions *Actions) FileRead(ctx context.Context, path string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "read", Path: path})
}

func (actions *Actions) FileWriteText(ctx context.Context, path, content string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "write", Path: path, ContentText: content})
}

func (actions *Actions) FileWriteBase64(ctx context.Context, path, content string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "write", Path: path, ContentBase64: content})
}

func (actions *Actions) FileDelete(ctx context.Context, path string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "delete", Path: path})
}

func (actions *Actions) FileList(ctx context.Context, prefix string) (ActionResult, error) {
	return actions.callResult(ctx, "storage.file", FileRequest{Operation: "list", Prefix: prefix})
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

type ConfigWriteRequest struct {
	Values map[string]any `json:"values"`
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

type ThirdPartyAccountObservation string

const (
	ThirdPartyAccountObservationAuthRejected   ThirdPartyAccountObservation = "auth_rejected"
	ThirdPartyAccountObservationSessionBlocked ThirdPartyAccountObservation = "session_blocked"
)

type ThirdPartyAccountValidateRequest struct {
	Platform    string                       `json:"platform"`
	AccountID   string                       `json:"account_id"`
	Observation ThirdPartyAccountObservation `json:"observation"`
	HTTPStatus  int                          `json:"http_status,omitempty"`
}

func (actions *Actions) ThirdPartyAccountValidate(ctx context.Context, request ThirdPartyAccountValidateRequest) (ActionResult, error) {
	return actions.callResult(ctx, "thirdparty.account.validate", request)
}

type ThirdPartyResolveRequest struct {
	Platform string `json:"platform"`
	Query    string `json:"query"`
	// Cookie 为账号 CK（敏感凭据），宿主用于恢复登录环境解析关键词。
	Cookie string `json:"cookie,omitempty"`
}

func (actions *Actions) ThirdPartyResolve(ctx context.Context, request ThirdPartyResolveRequest) (ActionResult, error) {
	return actions.callResult(ctx, "thirdparty.resolve", request)
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

type RenderImageRequest struct {
	Template     string                `json:"template"`
	Data         map[string]any        `json:"data"`
	Theme        string                `json:"theme,omitempty"`
	Output       string                `json:"output,omitempty"`
	FallbackText string                `json:"fallback_text,omitempty"`
	Resources    []RenderImageResource `json:"resources,omitempty"`
}

type RenderImageResource struct {
	ID           string   `json:"id"`
	URL          string   `json:"url"`
	FallbackURLs []string `json:"fallback_urls,omitempty"`
	Referer      string   `json:"referer,omitempty"`
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
