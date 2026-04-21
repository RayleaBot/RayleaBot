import type { EventBody, Segment } from './types.js';
export type { Frame, Segment, EventBody } from './types.js';
export { textSegment, imageSegment, atSegment, atAllSegment, faceSegment, replySegment, passthroughSegment, recordSegment, videoSegment, markdownSegment, fileSegment, flashFileSegment, jsonSegment, xmlSegment, musicSegment, contactSegment, forwardSegment, nodeSegment, pokeSegment, diceSegment, rpsSegment, mfaceSegment, keyboardSegment, shakeSegment, } from './types.js';
export { ActionError } from './protocol.js';
type EventHandler = (event: EventBody, requestId: string) => void | Promise<void>;
type ConversationType = 'group' | 'private';
type ProviderName = 'napcat' | 'luckylillia';
interface ActionOptions {
    timeoutMs?: number;
}
interface MessageForwardGetOptions extends ActionOptions {
    messageId?: string;
    forwardId?: string;
}
interface MessageReadMarkOptions extends ActionOptions {
    messageId?: string;
    conversationType?: ConversationType;
    conversationId?: string;
}
interface GroupBanSetOptions extends ActionOptions {
    userId?: string;
    durationSeconds?: number;
    wholeGroup?: boolean;
}
interface FileGroupFsListOptions extends ActionOptions {
    folderId?: string;
}
interface FileGroupFsDeleteOptions extends ActionOptions {
    folderId?: string;
    fileId?: string;
}
interface GovernanceBlacklistWriteOptions extends ActionOptions {
    entryType?: 'user' | 'group';
    targetId?: string;
    reason?: string;
}
interface GovernanceWhitelistWriteOptions extends ActionOptions {
    enabled?: boolean;
    entryType?: 'user' | 'group';
    targetId?: string;
    reason?: string;
}
export interface RayleaBotPlugin {
    readonly botId: string;
    readonly capabilities: string[];
    readonly commandPrefixes: string[];
    readonly primaryCommandPrefix: string;
    onEvent(handler: EventHandler): RayleaBotPlugin;
    onEvent(eventType: string, handler: EventHandler): RayleaBotPlugin;
    onCommand(name: string, handler: EventHandler, aliases?: string[]): RayleaBotPlugin;
    subscribe(...eventTypes: string[]): RayleaBotPlugin;
    sendMessage(requestId: string, targetType: string, targetId: string, segments: Segment[]): void;
    sendReply(requestId: string, replyToEventId: string, segments: Segment[], options?: {
        fallbackToSendIfMissing?: boolean;
    }): void;
    loggerWrite(requestId: string, level: string, message: string, fields?: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageGet(requestId: string, key: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageSet(requestId: string, key: string, value: unknown, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageDelete(requestId: string, key: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageList(requestId: string, prefix?: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    storageFileRead(requestId: string, path: string, options?: ActionOptions & {
        root?: string;
    }): Promise<Record<string, unknown>>;
    storageFileWrite(requestId: string, path: string, options?: ActionOptions & {
        root?: string;
        contentText?: string;
        contentBase64?: string;
    }): Promise<Record<string, unknown>>;
    storageFileDelete(requestId: string, path: string, options?: ActionOptions & {
        root?: string;
    }): Promise<Record<string, unknown>>;
    storageFileList(requestId: string, prefix?: string, options?: ActionOptions & {
        root?: string;
    }): Promise<Record<string, unknown>>;
    httpRequest(requestId: string, method: string, url: string, options?: ActionOptions & {
        headers?: Record<string, string>;
        timeoutSeconds?: number;
        bodyText?: string;
        bodyBase64?: string;
    }): Promise<Record<string, unknown>>;
    configRead(requestId: string, keys: string[], options?: ActionOptions): Promise<Record<string, unknown>>;
    configWrite(requestId: string, values: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    governanceBlacklistRead(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    governanceBlacklistWrite(requestId: string, operation: 'upsert' | 'delete', options?: GovernanceBlacklistWriteOptions): Promise<Record<string, unknown>>;
    governanceWhitelistRead(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    governanceWhitelistWrite(requestId: string, operation: 'set_enabled' | 'upsert' | 'delete', options?: GovernanceWhitelistWriteOptions): Promise<Record<string, unknown>>;
    governanceCommandPolicyRead(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    schedulerCreate(requestId: string, taskId: string, cron: string, options?: ActionOptions & {
        payload?: Record<string, unknown>;
    }): Promise<Record<string, unknown>>;
    exposeWebhook(requestId: string, route: string, options?: ActionOptions & {
        methods?: string[];
        authStrategy?: string;
        header?: string;
        secretRef: string;
        signaturePrefix?: string;
        sourceIps?: string[];
    }): Promise<Record<string, unknown>>;
    renderImage(requestId: string, template: string, data: Record<string, unknown>, options?: ActionOptions & {
        theme?: string;
        output?: string;
        fallbackText?: string;
    }): Promise<Record<string, unknown>>;
    pluginList(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    onebotAction(requestId: string, action: string, data?: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    providerAction(requestId: string, provider: ProviderName, action: string, data?: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    messageGet(requestId: string, messageId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    messageDelete(requestId: string, messageId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    messageHistoryGet(requestId: string, conversationType: ConversationType, conversationId: string, options?: ActionOptions & {
        limit?: number;
    }): Promise<Record<string, unknown>>;
    messageForwardGet(requestId: string, options?: MessageForwardGetOptions): Promise<Record<string, unknown>>;
    messageForwardSend(requestId: string, targetType: ConversationType, targetId: string, messages: Record<string, unknown>[], options?: ActionOptions): Promise<Record<string, unknown>>;
    messageReadMark(requestId: string, options?: MessageReadMarkOptions): Promise<Record<string, unknown>>;
    friendRequestHandle(requestId: string, flag: string, approve: boolean, options?: ActionOptions): Promise<Record<string, unknown>>;
    friendList(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    friendRemarkSet(requestId: string, userId: string, remark: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    userInfoGet(requestId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    userLikeSend(requestId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupList(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupInfoGet(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupMemberGet(requestId: string, groupId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupMemberList(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupRequestHandle(requestId: string, flag: string, approve: boolean, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupLeave(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupAdminSet(requestId: string, groupId: string, userId: string, enabled: boolean, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupBanSet(requestId: string, groupId: string, options?: GroupBanSetOptions): Promise<Record<string, unknown>>;
    groupCardSet(requestId: string, groupId: string, userId: string, card: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupTitleSet(requestId: string, groupId: string, userId: string, title: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupNameSet(requestId: string, groupId: string, name: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupAnnouncementList(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupAnnouncementCreate(requestId: string, groupId: string, content: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupAnnouncementDelete(requestId: string, groupId: string, noticeId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupEssenceList(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupEssenceSet(requestId: string, messageId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupEssenceUnset(requestId: string, messageId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupHonorGet(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    groupTodoSet(requestId: string, groupId: string, todo: Record<string, unknown>, options?: ActionOptions): Promise<Record<string, unknown>>;
    fileGet(requestId: string, fileId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    fileDownload(requestId: string, fileId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    fileGroupUpload(requestId: string, groupId: string, fileName: string, fileUrl: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    filePrivateUpload(requestId: string, userId: string, fileName: string, fileUrl: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    fileGroupUrlGet(requestId: string, groupId: string, fileId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    filePrivateUrlGet(requestId: string, userId: string, fileId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    fileGroupFsInfo(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    fileGroupFsList(requestId: string, groupId: string, options?: FileGroupFsListOptions): Promise<Record<string, unknown>>;
    fileGroupFsMkdir(requestId: string, groupId: string, name: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    fileGroupFsDelete(requestId: string, groupId: string, options?: FileGroupFsDeleteOptions): Promise<Record<string, unknown>>;
    reactionSet(requestId: string, messageId: string, emoji: string, enabled?: boolean, options?: ActionOptions): Promise<Record<string, unknown>>;
    reactionList(requestId: string, messageId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    pokeSend(requestId: string, targetType: ConversationType, targetId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    napcatMessageEmojiLikeSet(requestId: string, messageId: string, emojiId: string, enabled?: boolean, options?: ActionOptions): Promise<Record<string, unknown>>;
    napcatGroupSignSet(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    luckylilliaFriendGroupsGet(requestId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
    run(): Promise<void>;
}
export declare function createPlugin(): RayleaBotPlugin;
//# sourceMappingURL=index.d.ts.map