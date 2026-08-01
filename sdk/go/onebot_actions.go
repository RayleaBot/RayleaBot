package rayleabot

import (
	"context"
	"errors"
)

type ConversationType string

const (
	ConversationGroup   ConversationType = "group"
	ConversationPrivate ConversationType = "private"
)

type MessageHistoryGetRequest struct {
	ConversationType ConversationType `json:"conversation_type"`
	ConversationID   string           `json:"conversation_id"`
	Limit            *int             `json:"limit,omitempty"`
}

type MessageForwardGetRequest struct {
	MessageID string `json:"message_id,omitempty"`
	ForwardID string `json:"forward_id,omitempty"`
}

type ForwardMessage map[string]any

type MessageForwardSendRequest struct {
	TargetType ConversationType `json:"target_type"`
	TargetID   string           `json:"target_id"`
	Messages   []ForwardMessage `json:"messages"`
}

type MessageReadMarkRequest struct {
	MessageID        string           `json:"message_id,omitempty"`
	ConversationType ConversationType `json:"conversation_type,omitempty"`
	ConversationID   string           `json:"conversation_id,omitempty"`
}

type GroupBanSetRequest struct {
	GroupID         string `json:"group_id"`
	UserID          string `json:"user_id,omitempty"`
	DurationSeconds *int   `json:"duration_seconds,omitempty"`
	WholeGroup      bool   `json:"whole_group"`
}

type FileGroupFSListRequest struct {
	GroupID  string `json:"group_id"`
	FolderID string `json:"folder_id,omitempty"`
}

type FileGroupFSDeleteRequest struct {
	GroupID  string `json:"group_id"`
	FolderID string `json:"folder_id,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

type ReactionSetRequest struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
	Enabled   bool   `json:"enabled"`
}

type PokeSendRequest struct {
	TargetType ConversationType `json:"target_type"`
	TargetID   string           `json:"target_id"`
	UserID     string           `json:"user_id"`
}

type NapCatMessageEmojiLikeSetRequest struct {
	MessageID string `json:"message_id"`
	EmojiID   string `json:"emoji_id"`
	Enabled   bool   `json:"enabled"`
}

func (actions *Actions) oneBotResult(ctx context.Context, action OneBotAction, request any) (ActionResult, error) {
	return actions.callResult(ctx, string(action), request)
}

func (actions *Actions) providerResult(ctx context.Context, action ProviderAction, request any) (ActionResult, error) {
	return actions.callResult(ctx, string(action), request)
}

func (actions *Actions) MessageGet(ctx context.Context, messageID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionMessageGet, struct {
		MessageID string `json:"message_id"`
	}{messageID})
}

func (actions *Actions) MessageDelete(ctx context.Context, messageID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionMessageDelete, struct {
		MessageID string `json:"message_id"`
	}{messageID})
}

func (actions *Actions) MessageHistoryGet(ctx context.Context, request MessageHistoryGetRequest) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionMessageHistoryGet, request)
}

func (actions *Actions) MessageForwardGet(ctx context.Context, request MessageForwardGetRequest) (ActionResult, error) {
	if request.MessageID == "" && request.ForwardID == "" {
		return nil, errors.New("rayleabot: MessageForwardGet requires MessageID or ForwardID")
	}
	return actions.oneBotResult(ctx, ActionMessageForwardGet, request)
}

func (actions *Actions) MessageForwardSend(ctx context.Context, request MessageForwardSendRequest) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionMessageForwardSend, request)
}

func (actions *Actions) MessageReadMark(ctx context.Context, request MessageReadMarkRequest) (ActionResult, error) {
	if request.MessageID == "" && (request.ConversationType == "" || request.ConversationID == "") {
		return nil, errors.New("rayleabot: MessageReadMark requires MessageID or ConversationType with ConversationID")
	}
	return actions.oneBotResult(ctx, ActionMessageReadMark, request)
}

func (actions *Actions) FriendRequestHandle(ctx context.Context, flag string, approve bool) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFriendRequestHandle, struct {
		Flag    string `json:"flag"`
		Approve bool   `json:"approve"`
	}{flag, approve})
}

func (actions *Actions) FriendList(ctx context.Context) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFriendList, struct{}{})
}

func (actions *Actions) FriendRemarkSet(ctx context.Context, userID, remark string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFriendRemarkSet, struct {
		UserID string `json:"user_id"`
		Remark string `json:"remark"`
	}{userID, remark})
}

func (actions *Actions) UserInfoGet(ctx context.Context, userID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionUserInfoGet, struct {
		UserID string `json:"user_id"`
	}{userID})
}

func (actions *Actions) UserLikeSend(ctx context.Context, userID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionUserLikeSend, struct {
		UserID string `json:"user_id"`
	}{userID})
}

func (actions *Actions) GroupList(ctx context.Context) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupList, struct{}{})
}

func (actions *Actions) GroupInfoGet(ctx context.Context, groupID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupInfoGet, struct {
		GroupID string `json:"group_id"`
	}{groupID})
}

func (actions *Actions) GroupMemberGet(ctx context.Context, groupID, userID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupMemberGet, struct {
		GroupID string `json:"group_id"`
		UserID  string `json:"user_id"`
	}{groupID, userID})
}

func (actions *Actions) GroupMemberList(ctx context.Context, groupID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupMemberList, struct {
		GroupID string `json:"group_id"`
	}{groupID})
}

func (actions *Actions) GroupRequestHandle(ctx context.Context, flag string, approve bool) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupRequestHandle, struct {
		Flag    string `json:"flag"`
		Approve bool   `json:"approve"`
	}{flag, approve})
}

func (actions *Actions) GroupLeave(ctx context.Context, groupID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupLeave, struct {
		GroupID string `json:"group_id"`
	}{groupID})
}

func (actions *Actions) GroupAdminSet(ctx context.Context, groupID, userID string, enabled bool) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupAdminSet, struct {
		GroupID string `json:"group_id"`
		UserID  string `json:"user_id"`
		Enabled bool   `json:"enabled"`
	}{groupID, userID, enabled})
}

func (actions *Actions) GroupBanSet(ctx context.Context, request GroupBanSetRequest) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupBanSet, request)
}

func (actions *Actions) GroupCardSet(ctx context.Context, groupID, userID, card string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupCardSet, struct {
		GroupID string `json:"group_id"`
		UserID  string `json:"user_id"`
		Card    string `json:"card"`
	}{groupID, userID, card})
}

func (actions *Actions) GroupTitleSet(ctx context.Context, groupID, userID, title string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupTitleSet, struct {
		GroupID string `json:"group_id"`
		UserID  string `json:"user_id"`
		Title   string `json:"title"`
	}{groupID, userID, title})
}

func (actions *Actions) GroupNameSet(ctx context.Context, groupID, name string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupNameSet, struct {
		GroupID string `json:"group_id"`
		Name    string `json:"name"`
	}{groupID, name})
}

func (actions *Actions) GroupAnnouncementList(ctx context.Context, groupID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupAnnouncementList, struct {
		GroupID string `json:"group_id"`
	}{groupID})
}

func (actions *Actions) GroupAnnouncementCreate(ctx context.Context, groupID, content string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupAnnouncementCreate, struct {
		GroupID string `json:"group_id"`
		Content string `json:"content"`
	}{groupID, content})
}

func (actions *Actions) GroupAnnouncementDelete(ctx context.Context, groupID, noticeID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupAnnouncementDelete, struct {
		GroupID  string `json:"group_id"`
		NoticeID string `json:"notice_id"`
	}{groupID, noticeID})
}

func (actions *Actions) GroupEssenceList(ctx context.Context, groupID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupEssenceList, struct {
		GroupID string `json:"group_id"`
	}{groupID})
}

func (actions *Actions) GroupEssenceSet(ctx context.Context, messageID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupEssenceSet, struct {
		MessageID string `json:"message_id"`
	}{messageID})
}

func (actions *Actions) GroupEssenceUnset(ctx context.Context, messageID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupEssenceUnset, struct {
		MessageID string `json:"message_id"`
	}{messageID})
}

func (actions *Actions) GroupHonorGet(ctx context.Context, groupID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupHonorGet, struct {
		GroupID string `json:"group_id"`
	}{groupID})
}

func (actions *Actions) GroupTodoSet(ctx context.Context, groupID string, todo map[string]any) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionGroupTodoSet, struct {
		GroupID string         `json:"group_id"`
		Todo    map[string]any `json:"todo"`
	}{groupID, todo})
}

func (actions *Actions) FileGet(ctx context.Context, fileID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFileGet, struct {
		FileID string `json:"file_id"`
	}{fileID})
}

func (actions *Actions) FileDownload(ctx context.Context, fileID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFileDownload, struct {
		FileID string `json:"file_id"`
	}{fileID})
}

func (actions *Actions) FileGroupUpload(ctx context.Context, groupID, fileName, fileURL string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFileGroupUpload, struct {
		GroupID  string `json:"group_id"`
		FileName string `json:"file_name"`
		FileURL  string `json:"file_url"`
	}{groupID, fileName, fileURL})
}

func (actions *Actions) FilePrivateUpload(ctx context.Context, userID, fileName, fileURL string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFilePrivateUpload, struct {
		UserID   string `json:"user_id"`
		FileName string `json:"file_name"`
		FileURL  string `json:"file_url"`
	}{userID, fileName, fileURL})
}

func (actions *Actions) FileGroupURLGet(ctx context.Context, groupID, fileID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFileGroupURLGet, struct {
		GroupID string `json:"group_id"`
		FileID  string `json:"file_id"`
	}{groupID, fileID})
}

func (actions *Actions) FilePrivateURLGet(ctx context.Context, userID, fileID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFilePrivateURLGet, struct {
		UserID string `json:"user_id"`
		FileID string `json:"file_id"`
	}{userID, fileID})
}

func (actions *Actions) FileGroupFSInfo(ctx context.Context, groupID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFileGroupFSInfo, struct {
		GroupID string `json:"group_id"`
	}{groupID})
}

func (actions *Actions) FileGroupFSList(ctx context.Context, request FileGroupFSListRequest) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFileGroupFSList, request)
}

func (actions *Actions) FileGroupFSMkdir(ctx context.Context, groupID, name string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionFileGroupFSMkdir, struct {
		GroupID string `json:"group_id"`
		Name    string `json:"name"`
	}{groupID, name})
}

func (actions *Actions) FileGroupFSDelete(ctx context.Context, request FileGroupFSDeleteRequest) (ActionResult, error) {
	if request.FolderID == "" && request.FileID == "" {
		return nil, errors.New("rayleabot: FileGroupFSDelete requires FolderID or FileID")
	}
	return actions.oneBotResult(ctx, ActionFileGroupFSDelete, request)
}

func (actions *Actions) ReactionSet(ctx context.Context, request ReactionSetRequest) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionReactionSet, request)
}

func (actions *Actions) ReactionList(ctx context.Context, messageID string) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionReactionList, struct {
		MessageID string `json:"message_id"`
	}{messageID})
}

func (actions *Actions) PokeSend(ctx context.Context, request PokeSendRequest) (ActionResult, error) {
	return actions.oneBotResult(ctx, ActionPokeSend, request)
}

func (actions *Actions) NapCatMessageEmojiLikeSet(ctx context.Context, request NapCatMessageEmojiLikeSetRequest) (ActionResult, error) {
	return actions.providerResult(ctx, ActionNapCatMessageEmojiLikeSet, request)
}

func (actions *Actions) NapCatGroupSignSet(ctx context.Context, groupID string) (ActionResult, error) {
	return actions.providerResult(ctx, ActionNapCatGroupSignSet, struct {
		GroupID string `json:"group_id"`
	}{groupID})
}

func (actions *Actions) LuckyLilliaFriendGroupsGet(ctx context.Context, userID string) (ActionResult, error) {
	return actions.providerResult(ctx, ActionLuckyLilliaFriendGroupsGet, struct {
		UserID string `json:"user_id"`
	}{userID})
}
