import type {
  EventBody,
  Frame,
  InitFrame,
  EventFrame,
  Segment,
} from './types.js';
import {
  readFrames,
  requestLocalAction,
  sendAction,
  sendError,
  sendInitAck,
  sendPong,
  sendResult,
} from './protocol.js';

export type { Frame, Segment, EventBody } from './types.js';
export {
  textSegment,
  imageSegment,
  atSegment,
  atAllSegment,
  faceSegment,
  replySegment,
  passthroughSegment,
  recordSegment,
  videoSegment,
  markdownSegment,
  fileSegment,
  flashFileSegment,
  jsonSegment,
  xmlSegment,
  musicSegment,
  contactSegment,
  forwardSegment,
  nodeSegment,
  pokeSegment,
  diceSegment,
  rpsSegment,
  mfaceSegment,
  keyboardSegment,
  shakeSegment,
} from './types.js';
export { ActionError } from './protocol.js';

type LegacyEventHandler = (event: EventBody, requestId: string) => void | Promise<void>;
type ContextEventHandler = (ctx: PluginEventContext) => void | Promise<void>;
type EventHandler = LegacyEventHandler | ContextEventHandler;
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

export interface RayleaBotPluginRuntime {
  readonly botId: string;
  readonly capabilities: string[];
  readonly superAdmins: string[];
  readonly commandPrefixes: string[];
  readonly primaryCommandPrefix: string;

  awaitBotIdentity(timeoutMs?: number): Promise<string>;

  onEvent(handler: EventHandler): RayleaBotPluginRuntime;
  onEvent(eventType: string, handler: EventHandler): RayleaBotPluginRuntime;
  onCommand(name: string, handler: EventHandler, aliases?: string[]): RayleaBotPluginRuntime;
  subscribe(...eventTypes: string[]): RayleaBotPluginRuntime;

  sendMessage(requestId: string, targetType: string, targetId: string, segments: Segment[]): void;
  sendReply(
    requestId: string,
    replyToEventId: string,
    segments: Segment[],
    options?: { fallbackToSendIfMissing?: boolean },
  ): void;
  sendResult(requestId: string, data?: Record<string, unknown>): void;

  loggerWrite(
    requestId: string,
    level: string,
    message: string,
    fields?: Record<string, unknown>,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  storageGet(requestId: string, key: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  storageSet(
    requestId: string,
    key: string,
    value: unknown,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  storageDelete(requestId: string, key: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  storageList(requestId: string, prefix?: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  storageFileRead(
    requestId: string,
    path: string,
    options?: ActionOptions & { root?: string },
  ): Promise<Record<string, unknown>>;
  storageFileWrite(
    requestId: string,
    path: string,
    options?: ActionOptions & { root?: string; contentText?: string; contentBase64?: string },
  ): Promise<Record<string, unknown>>;
  storageFileDelete(
    requestId: string,
    path: string,
    options?: ActionOptions & { root?: string },
  ): Promise<Record<string, unknown>>;
  storageFileList(
    requestId: string,
    prefix?: string,
    options?: ActionOptions & { root?: string },
  ): Promise<Record<string, unknown>>;
  httpRequest(
    requestId: string,
    method: string,
    url: string,
    options?: ActionOptions & {
      headers?: Record<string, string>;
      timeoutSeconds?: number;
      bodyText?: string;
      bodyBase64?: string;
    },
  ): Promise<Record<string, unknown>>;
  configRead(
    requestId: string,
    keys: string[],
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  configWrite(
    requestId: string,
    values: Record<string, unknown>,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  governanceBlacklistRead(
    requestId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  governanceBlacklistWrite(
    requestId: string,
    operation: 'upsert' | 'delete',
    options?: GovernanceBlacklistWriteOptions,
  ): Promise<Record<string, unknown>>;
  governanceWhitelistRead(
    requestId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  governanceWhitelistWrite(
    requestId: string,
    operation: 'set_enabled' | 'upsert' | 'delete',
    options?: GovernanceWhitelistWriteOptions,
  ): Promise<Record<string, unknown>>;
  governanceCommandPolicyRead(
    requestId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  schedulerCreate(
    requestId: string,
    taskId: string,
    cron: string,
    options?: ActionOptions & { payload?: Record<string, unknown>; logLabel?: string },
  ): Promise<Record<string, unknown>>;
  exposeWebhook(
    requestId: string,
    route: string,
    options?: ActionOptions & {
      methods?: string[];
      authStrategy?: string;
      header?: string;
      secretRef: string;
      signaturePrefix?: string;
      sourceIps?: string[];
      replayProtection?: {
        timestampHeader?: string;
        eventIdHeader?: string;
        toleranceSeconds?: number;
        enforce?: boolean;
      };
    },
  ): Promise<Record<string, unknown>>;
  renderImage(
    requestId: string,
    template: string,
    data: Record<string, unknown>,
    options?: ActionOptions & {
      theme?: string;
      output?: string;
      fallbackText?: string;
    },
  ): Promise<Record<string, unknown>>;
  pluginList(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  onebotAction(
    requestId: string,
    action: string,
    data?: Record<string, unknown>,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  providerAction(
    requestId: string,
    provider: ProviderName,
    action: string,
    data?: Record<string, unknown>,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  messageGet(requestId: string, messageId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  messageDelete(
    requestId: string,
    messageId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  messageHistoryGet(
    requestId: string,
    conversationType: ConversationType,
    conversationId: string,
    options?: ActionOptions & { limit?: number },
  ): Promise<Record<string, unknown>>;
  messageForwardGet(
    requestId: string,
    options?: MessageForwardGetOptions,
  ): Promise<Record<string, unknown>>;
  messageForwardSend(
    requestId: string,
    targetType: ConversationType,
    targetId: string,
    messages: Record<string, unknown>[],
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  messageReadMark(
    requestId: string,
    options?: MessageReadMarkOptions,
  ): Promise<Record<string, unknown>>;
  friendRequestHandle(
    requestId: string,
    flag: string,
    approve: boolean,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  friendList(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  friendRemarkSet(
    requestId: string,
    userId: string,
    remark: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  userInfoGet(requestId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  userLikeSend(requestId: string, userId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  groupList(requestId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  groupInfoGet(
    requestId: string,
    groupId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupMemberGet(
    requestId: string,
    groupId: string,
    userId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupMemberList(
    requestId: string,
    groupId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupRequestHandle(
    requestId: string,
    flag: string,
    approve: boolean,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupLeave(requestId: string, groupId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  groupAdminSet(
    requestId: string,
    groupId: string,
    userId: string,
    enabled: boolean,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupBanSet(
    requestId: string,
    groupId: string,
    options?: GroupBanSetOptions,
  ): Promise<Record<string, unknown>>;
  groupCardSet(
    requestId: string,
    groupId: string,
    userId: string,
    card: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupTitleSet(
    requestId: string,
    groupId: string,
    userId: string,
    title: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupNameSet(
    requestId: string,
    groupId: string,
    name: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupAnnouncementList(
    requestId: string,
    groupId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupAnnouncementCreate(
    requestId: string,
    groupId: string,
    content: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupAnnouncementDelete(
    requestId: string,
    groupId: string,
    noticeId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupEssenceList(
    requestId: string,
    groupId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupEssenceSet(
    requestId: string,
    messageId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupEssenceUnset(
    requestId: string,
    messageId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupHonorGet(
    requestId: string,
    groupId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  groupTodoSet(
    requestId: string,
    groupId: string,
    todo: Record<string, unknown>,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  fileGet(requestId: string, fileId: string, options?: ActionOptions): Promise<Record<string, unknown>>;
  fileDownload(
    requestId: string,
    fileId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  fileGroupUpload(
    requestId: string,
    groupId: string,
    fileName: string,
    fileUrl: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  filePrivateUpload(
    requestId: string,
    userId: string,
    fileName: string,
    fileUrl: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  fileGroupUrlGet(
    requestId: string,
    groupId: string,
    fileId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  filePrivateUrlGet(
    requestId: string,
    userId: string,
    fileId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  fileGroupFsInfo(
    requestId: string,
    groupId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  fileGroupFsList(
    requestId: string,
    groupId: string,
    options?: FileGroupFsListOptions,
  ): Promise<Record<string, unknown>>;
  fileGroupFsMkdir(
    requestId: string,
    groupId: string,
    name: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  fileGroupFsDelete(
    requestId: string,
    groupId: string,
    options?: FileGroupFsDeleteOptions,
  ): Promise<Record<string, unknown>>;
  reactionSet(
    requestId: string,
    messageId: string,
    emoji: string,
    enabled?: boolean,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  reactionList(
    requestId: string,
    messageId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  pokeSend(
    requestId: string,
    targetType: ConversationType,
    targetId: string,
    userId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  napcatMessageEmojiLikeSet(
    requestId: string,
    messageId: string,
    emojiId: string,
    enabled?: boolean,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  napcatGroupSignSet(
    requestId: string,
    groupId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;
  luckylilliaFriendGroupsGet(
    requestId: string,
    userId: string,
    options?: ActionOptions,
  ): Promise<Record<string, unknown>>;

  run(): Promise<void>;
}

export class PluginEventContext {
  readonly event: EventBody;
  readonly requestId: string;
  private readonly plugin: RayleaBotPluginRuntime;

  constructor(plugin: RayleaBotPluginRuntime, event: EventBody, requestId: string) {
    this.plugin = plugin;
    this.event = event;
    this.requestId = requestId;
  }

  get payload(): EventBody['payload'] {
    return this.event.payload ?? {};
  }

  get target(): EventBody['target'] {
    return this.event.target;
  }

  get actor(): EventBody['actor'] {
    return this.event.actor;
  }

  get message(): EventBody['message'] {
    return this.event.message ?? {};
  }

  get eventType(): string {
    return this.event.event_type;
  }

  get command(): string | null | undefined {
    return this.event.payload?.command;
  }

  get args(): string[] {
    return this.event.payload?.args ?? [];
  }

  get plainText(): string {
    return this.event.message?.plain_text ?? '';
  }

  get targetType(): string {
    return this.event.target?.type ?? 'group';
  }

  get targetId(): string {
    return this.event.target?.id ?? '';
  }

  get botId(): string {
    return this.plugin.botId;
  }

  awaitBotIdentity(timeoutMs?: number): Promise<string> {
    return this.plugin.awaitBotIdentity(timeoutMs);
  }

  get capabilities(): string[] {
    return this.plugin.capabilities;
  }

  get superAdmins(): string[] {
    return this.plugin.superAdmins;
  }

  get commandPrefixes(): string[] {
    return this.plugin.commandPrefixes;
  }

  get primaryCommandPrefix(): string {
    return this.plugin.primaryCommandPrefix;
  }

  sendMessage(segments: Segment[], options: { targetType?: string; targetId?: string } = {}): void {
    this.plugin.sendMessage(
      this.requestId,
      options.targetType ?? this.targetType,
      options.targetId ?? this.targetId,
      segments,
    );
  }

  sendText(text: string, options: { targetType?: string; targetId?: string } = {}): void {
    this.sendMessage([{ type: 'text', data: { text } }], options);
  }

  sendReply(
    replyToEventId: string,
    segments: Segment[],
    options: { fallbackToSendIfMissing?: boolean } = {},
  ): void {
    this.plugin.sendReply(this.requestId, replyToEventId, segments, options);
  }

  sendResult(data: Record<string, unknown> = {}): void {
    this.plugin.sendResult(this.requestId, data);
  }

  loggerWrite(
    level: string,
    message: string,
    fields?: Record<string, unknown>,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.loggerWrite(this.requestId, level, message, fields, options);
  }

  storageGet(key: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.storageGet(this.requestId, key, options);
  }

  storageSet(key: string, value: unknown, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.storageSet(this.requestId, key, value, options);
  }

  storageDelete(key: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.storageDelete(this.requestId, key, options);
  }

  storageList(prefix = '', options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.storageList(this.requestId, prefix, options);
  }

  storageFileRead(
    path: string,
    options: ActionOptions & { root?: string } = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.storageFileRead(this.requestId, path, options);
  }

  storageFileWrite(
    path: string,
    options: ActionOptions & { root?: string; contentText?: string; contentBase64?: string } = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.storageFileWrite(this.requestId, path, options);
  }

  storageFileDelete(
    path: string,
    options: ActionOptions & { root?: string } = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.storageFileDelete(this.requestId, path, options);
  }

  storageFileList(
    prefix = '',
    options: ActionOptions & { root?: string } = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.storageFileList(this.requestId, prefix, options);
  }

  httpRequest(
    method: string,
    url: string,
    options: ActionOptions & {
      headers?: Record<string, string>;
      timeoutSeconds?: number;
      bodyText?: string;
      bodyBase64?: string;
    } = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.httpRequest(this.requestId, method, url, options);
  }

  configRead(keys: string[], options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.configRead(this.requestId, keys, options);
  }

  configWrite(values: Record<string, unknown>, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.configWrite(this.requestId, values, options);
  }

  governanceBlacklistRead(options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.governanceBlacklistRead(this.requestId, options);
  }

  governanceBlacklistWrite(
    operation: 'upsert' | 'delete',
    options: GovernanceBlacklistWriteOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.governanceBlacklistWrite(this.requestId, operation, options);
  }

  governanceWhitelistRead(options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.governanceWhitelistRead(this.requestId, options);
  }

  governanceWhitelistWrite(
    operation: 'set_enabled' | 'upsert' | 'delete',
    options: GovernanceWhitelistWriteOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.governanceWhitelistWrite(this.requestId, operation, options);
  }

  governanceCommandPolicyRead(options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.governanceCommandPolicyRead(this.requestId, options);
  }

  schedulerCreate(
    taskId: string,
    cron: string,
    options: ActionOptions & { payload?: Record<string, unknown>; logLabel?: string } = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.schedulerCreate(this.requestId, taskId, cron, options);
  }

  exposeWebhook(
    route: string,
    options: ActionOptions & {
      methods?: string[];
      authStrategy?: string;
      header?: string;
      secretRef: string;
      signaturePrefix?: string;
      sourceIps?: string[];
      replayProtection?: {
        timestampHeader?: string;
        eventIdHeader?: string;
        toleranceSeconds?: number;
        enforce?: boolean;
      };
    },
  ): Promise<Record<string, unknown>> {
    return this.plugin.exposeWebhook(this.requestId, route, options);
  }

  renderImage(
    template: string,
    data: Record<string, unknown>,
    options: ActionOptions & {
      theme?: string;
      output?: string;
      fallbackText?: string;
    } = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.renderImage(this.requestId, template, data, options);
  }

  pluginList(options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.pluginList(this.requestId, options);
  }

  messageGet(messageId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.messageGet(this.requestId, messageId, options);
  }

  messageDelete(messageId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.messageDelete(this.requestId, messageId, options);
  }

  messageHistoryGet(
    conversationType: ConversationType,
    conversationId: string,
    options: ActionOptions & { limit?: number } = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.messageHistoryGet(this.requestId, conversationType, conversationId, options);
  }

  messageForwardGet(options: MessageForwardGetOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.messageForwardGet(this.requestId, options);
  }

  messageForwardSend(
    targetType: ConversationType,
    targetId: string,
    messages: Record<string, unknown>[],
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.messageForwardSend(this.requestId, targetType, targetId, messages, options);
  }

  messageReadMark(options: MessageReadMarkOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.messageReadMark(this.requestId, options);
  }

  friendRequestHandle(
    flag: string,
    approve: boolean,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.friendRequestHandle(this.requestId, flag, approve, options);
  }

  friendList(options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.friendList(this.requestId, options);
  }

  friendRemarkSet(userId: string, remark: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.friendRemarkSet(this.requestId, userId, remark, options);
  }

  userInfoGet(userId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.userInfoGet(this.requestId, userId, options);
  }

  userLikeSend(userId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.userLikeSend(this.requestId, userId, options);
  }

  groupList(options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupList(this.requestId, options);
  }

  groupInfoGet(groupId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupInfoGet(this.requestId, groupId, options);
  }

  groupMemberGet(
    groupId: string,
    userId: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.groupMemberGet(this.requestId, groupId, userId, options);
  }

  groupMemberList(groupId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupMemberList(this.requestId, groupId, options);
  }

  groupRequestHandle(
    flag: string,
    approve: boolean,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.groupRequestHandle(this.requestId, flag, approve, options);
  }

  groupLeave(groupId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupLeave(this.requestId, groupId, options);
  }

  groupAdminSet(
    groupId: string,
    userId: string,
    enabled: boolean,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.groupAdminSet(this.requestId, groupId, userId, enabled, options);
  }

  groupBanSet(groupId: string, options: GroupBanSetOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupBanSet(this.requestId, groupId, options);
  }

  groupCardSet(
    groupId: string,
    userId: string,
    card: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.groupCardSet(this.requestId, groupId, userId, card, options);
  }

  groupTitleSet(
    groupId: string,
    userId: string,
    title: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.groupTitleSet(this.requestId, groupId, userId, title, options);
  }

  groupNameSet(groupId: string, name: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupNameSet(this.requestId, groupId, name, options);
  }

  groupAnnouncementList(groupId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupAnnouncementList(this.requestId, groupId, options);
  }

  groupAnnouncementCreate(
    groupId: string,
    content: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.groupAnnouncementCreate(this.requestId, groupId, content, options);
  }

  groupAnnouncementDelete(
    groupId: string,
    noticeId: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.groupAnnouncementDelete(this.requestId, groupId, noticeId, options);
  }

  groupEssenceList(groupId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupEssenceList(this.requestId, groupId, options);
  }

  groupEssenceSet(messageId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupEssenceSet(this.requestId, messageId, options);
  }

  groupEssenceUnset(messageId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupEssenceUnset(this.requestId, messageId, options);
  }

  groupHonorGet(groupId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.groupHonorGet(this.requestId, groupId, options);
  }

  groupTodoSet(
    groupId: string,
    todo: Record<string, unknown>,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.groupTodoSet(this.requestId, groupId, todo, options);
  }

  fileGet(fileId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.fileGet(this.requestId, fileId, options);
  }

  fileDownload(fileId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.fileDownload(this.requestId, fileId, options);
  }

  fileGroupUpload(
    groupId: string,
    fileName: string,
    fileUrl: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.fileGroupUpload(this.requestId, groupId, fileName, fileUrl, options);
  }

  filePrivateUpload(
    userId: string,
    fileName: string,
    fileUrl: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.filePrivateUpload(this.requestId, userId, fileName, fileUrl, options);
  }

  fileGroupUrlGet(
    groupId: string,
    fileId: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.fileGroupUrlGet(this.requestId, groupId, fileId, options);
  }

  filePrivateUrlGet(
    userId: string,
    fileId: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.filePrivateUrlGet(this.requestId, userId, fileId, options);
  }

  fileGroupFsInfo(groupId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.fileGroupFsInfo(this.requestId, groupId, options);
  }

  fileGroupFsList(
    groupId: string,
    options: FileGroupFsListOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.fileGroupFsList(this.requestId, groupId, options);
  }

  fileGroupFsMkdir(groupId: string, name: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.fileGroupFsMkdir(this.requestId, groupId, name, options);
  }

  fileGroupFsDelete(
    groupId: string,
    options: FileGroupFsDeleteOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.fileGroupFsDelete(this.requestId, groupId, options);
  }

  reactionSet(
    messageId: string,
    emoji: string,
    enabled = true,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.reactionSet(this.requestId, messageId, emoji, enabled, options);
  }

  reactionList(messageId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.reactionList(this.requestId, messageId, options);
  }

  pokeSend(
    targetType: ConversationType,
    targetId: string,
    userId: string,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.pokeSend(this.requestId, targetType, targetId, userId, options);
  }

  napcatMessageEmojiLikeSet(
    messageId: string,
    emojiId: string,
    enabled = true,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.napcatMessageEmojiLikeSet(this.requestId, messageId, emojiId, enabled, options);
  }

  napcatGroupSignSet(groupId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.napcatGroupSignSet(this.requestId, groupId, options);
  }

  luckylilliaFriendGroupsGet(userId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
    return this.plugin.luckylilliaFriendGroupsGet(this.requestId, userId, options);
  }

  onebotAction(
    action: string,
    data: Record<string, unknown> = {},
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.onebotAction(this.requestId, action, data, options);
  }

  providerAction(
    provider: ProviderName,
    action: string,
    data: Record<string, unknown> = {},
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return this.plugin.providerAction(this.requestId, provider, action, data, options);
  }
}

function createPluginRuntime(owner?: RayleaBotPlugin): RayleaBotPluginRuntime {
  const eventHandlers: Array<{ type: string | null; handler: EventHandler }> = [];
  const commandHandlers = new Map<string, EventHandler>();
  const activeHandlers = new Set<Promise<void>>();
  let pluginId = '';
  let botId = '';
  let capabilities: string[] = [];
  let superAdmins: string[] = [];
  let commandPrefixes = ['/'];
  let subscriptions: string[] | null = null;
  const botIdentityWaiters = new Set<(value: string) => void>();

  function setBotId(next: string): void {
    botId = next || '';
    if (botId) {
      const current = botId;
      const pending = Array.from(botIdentityWaiters);
      botIdentityWaiters.clear();
      for (const resolve of pending) {
        try {
          resolve(current);
        } catch {
          // resolver throwing is the caller's bug; swallowing keeps the
          // event loop healthy for other waiters.
        }
      }
    }
  }

  function awaitBotIdentityImpl(timeoutMs: number): Promise<string> {
    if (botId) {
      return Promise.resolve(botId);
    }
    const clampedTimeout = Math.max(0, Math.floor(timeoutMs));
    return new Promise<string>((resolve) => {
      let settled = false;
      let timer: ReturnType<typeof setTimeout> | undefined;
      const wrappedResolve = (value: string) => finish(value);
      const finish = (value: string) => {
        if (settled) return;
        settled = true;
        botIdentityWaiters.delete(wrappedResolve);
        if (timer) {
          clearTimeout(timer);
        }
        resolve(value);
      };
      botIdentityWaiters.add(wrappedResolve);
      if (clampedTimeout > 0) {
        timer = setTimeout(() => finish(botId), clampedTimeout);
        if (typeof timer === 'object' && timer && 'unref' in timer && typeof (timer as { unref?: () => void }).unref === 'function') {
          (timer as { unref: () => void }).unref();
        }
      } else {
        // Zero timeout: keep waiting indefinitely.
      }
    });
  }

  const plugin: RayleaBotPluginRuntime = {
    get botId(): string {
      return botId;
    },

    awaitBotIdentity(timeoutMs: number = 30_000): Promise<string> {
      return awaitBotIdentityImpl(timeoutMs);
    },

    get capabilities(): string[] {
      return [...capabilities];
    },

    get superAdmins(): string[] {
      return [...superAdmins];
    },

    get commandPrefixes(): string[] {
      return [...commandPrefixes];
    },

    get primaryCommandPrefix(): string {
      return commandPrefixes[0] || '/';
    },

    onEvent(eventTypeOrHandler: string | EventHandler, handler?: EventHandler): RayleaBotPluginRuntime {
      if (typeof eventTypeOrHandler === 'function') {
        eventHandlers.push({ type: null, handler: eventTypeOrHandler });
      } else {
        eventHandlers.push({ type: eventTypeOrHandler, handler: handler! });
      }
      return plugin;
    },

    onCommand(name: string, handler: EventHandler, aliases: string[] = []): RayleaBotPluginRuntime {
      commandHandlers.set(name, handler);
      for (const alias of aliases) {
        commandHandlers.set(alias, handler);
      }
      return plugin;
    },

    subscribe(...eventTypes: string[]): RayleaBotPluginRuntime {
      subscriptions = eventTypes;
      return plugin;
    },

    sendMessage(
      requestId: string,
      targetType: string,
      targetId: string,
      segments: Segment[],
    ): void {
      sendAction(pluginId, requestId, 'message.send', {
        target_type: targetType,
        target_id: targetId,
        message: { segments },
      });
    },

    sendReply(
      requestId: string,
      replyToEventId: string,
      segments: Segment[],
      options: { fallbackToSendIfMissing?: boolean } = {},
    ): void {
      const data: Record<string, unknown> = {
        reply_to_event_id: replyToEventId,
        message: { segments },
      };
      if (options.fallbackToSendIfMissing) {
        data.fallback_to_send_if_missing = true;
      }
      sendAction(pluginId, requestId, 'message.reply', data);
    },

    sendResult(requestId: string, data: Record<string, unknown> = {}): void {
      sendResult(pluginId, requestId, data);
    },

    async loggerWrite(
      requestId: string,
      level: string,
      message: string,
      fields?: Record<string, unknown>,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      const data: Record<string, unknown> = { level, message };
      if (fields !== undefined) {
        data.fields = fields;
      }
      return await requestLocalAction(pluginId, requestId, 'logger.write', data, options);
    },

    async storageGet(
      requestId: string,
      key: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestLocalAction(
        pluginId,
        requestId,
        'storage.kv',
        { operation: 'get', key },
        options,
      );
    },

    async storageSet(
      requestId: string,
      key: string,
      value: unknown,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestLocalAction(
        pluginId,
        requestId,
        'storage.kv',
        { operation: 'set', key, value },
        options,
      );
    },

    async storageDelete(
      requestId: string,
      key: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestLocalAction(
        pluginId,
        requestId,
        'storage.kv',
        { operation: 'delete', key },
        options,
      );
    },

    async storageList(
      requestId: string,
      prefix = '',
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestLocalAction(
        pluginId,
        requestId,
        'storage.kv',
        { operation: 'list', prefix },
        options,
      );
    },

    async storageFileRead(
      requestId: string,
      path: string,
      options: ActionOptions & { root?: string } = {},
    ): Promise<Record<string, unknown>> {
      const root = options.root ?? 'plugin_data';
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(
        pluginId,
        requestId,
        'storage.file',
        { operation: 'read', root, path },
        { timeoutMs },
      );
    },

    async storageFileWrite(
      requestId: string,
      path: string,
      options: ActionOptions & { root?: string; contentText?: string; contentBase64?: string } = {},
    ): Promise<Record<string, unknown>> {
      const { root = 'plugin_data', contentText, contentBase64, timeoutMs = 30000 } = options;
      if ((contentText === undefined) === (contentBase64 === undefined)) {
        throw new Error('storageFileWrite requires exactly one of contentText or contentBase64');
      }
      const data: Record<string, unknown> = { operation: 'write', root, path };
      if (contentText !== undefined) {
        data.content_text = contentText;
      } else {
        data.content_base64 = contentBase64;
      }
      return await requestLocalAction(pluginId, requestId, 'storage.file', data, { timeoutMs });
    },

    async storageFileDelete(
      requestId: string,
      path: string,
      options: ActionOptions & { root?: string } = {},
    ): Promise<Record<string, unknown>> {
      const root = options.root ?? 'plugin_data';
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(
        pluginId,
        requestId,
        'storage.file',
        { operation: 'delete', root, path },
        { timeoutMs },
      );
    },

    async storageFileList(
      requestId: string,
      prefix = '',
      options: ActionOptions & { root?: string } = {},
    ): Promise<Record<string, unknown>> {
      const root = options.root ?? 'plugin_data';
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(
        pluginId,
        requestId,
        'storage.file',
        { operation: 'list', root, prefix },
        { timeoutMs },
      );
    },

    async httpRequest(
      requestId: string,
      method: string,
      url: string,
      options: ActionOptions & {
        headers?: Record<string, string>;
        timeoutSeconds?: number;
        bodyText?: string;
        bodyBase64?: string;
      } = {},
    ): Promise<Record<string, unknown>> {
      const { headers, timeoutMs = 30000, timeoutSeconds, bodyText, bodyBase64 } = options;
      if (bodyText !== undefined && bodyBase64 !== undefined) {
        throw new Error('httpRequest requires at most one of bodyText or bodyBase64');
      }
      const data: Record<string, unknown> = { method, url };
      if (headers !== undefined) {
        data.headers = headers;
      }
      if (timeoutSeconds !== undefined) {
        data.timeout_seconds = timeoutSeconds;
      }
      if (bodyText !== undefined) {
        data.body_text = bodyText;
      }
      if (bodyBase64 !== undefined) {
        data.body_base64 = bodyBase64;
      }
      return await requestLocalAction(pluginId, requestId, 'http.request', data, { timeoutMs });
    },

    async configRead(
      requestId: string,
      keys: string[],
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      if (!Array.isArray(keys) || keys.length === 0) {
        throw new Error('configRead requires at least one key');
      }
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(
        pluginId,
        requestId,
        'config.read',
        { keys },
        { timeoutMs },
      );
    },

    async configWrite(
      requestId: string,
      values: Record<string, unknown>,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      if (!values || typeof values !== 'object' || Object.keys(values).length === 0) {
        throw new Error('configWrite requires at least one key/value pair');
      }
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(
        pluginId,
        requestId,
        'config.write',
        { values },
        { timeoutMs },
      );
    },

    async governanceBlacklistRead(
      requestId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(pluginId, requestId, 'governance.blacklist.read', {}, { timeoutMs });
    },

    async governanceBlacklistWrite(
      requestId: string,
      operation: 'upsert' | 'delete',
      options: GovernanceBlacklistWriteOptions = {},
    ): Promise<Record<string, unknown>> {
      const { entryType, targetId, reason, timeoutMs = 30000 } = options;
      const data: Record<string, unknown> = { operation };
      if (operation === 'upsert') {
        if (!entryType || !targetId || !reason) {
          throw new Error('governanceBlacklistWrite upsert requires entryType, targetId, and reason');
        }
        data.entry_type = entryType;
        data.target_id = targetId;
        data.reason = reason;
      } else if (operation === 'delete') {
        if (!entryType || !targetId) {
          throw new Error('governanceBlacklistWrite delete requires entryType and targetId');
        }
        data.entry_type = entryType;
        data.target_id = targetId;
      } else {
        throw new Error('governanceBlacklistWrite requires operation upsert or delete');
      }
      return await requestLocalAction(pluginId, requestId, 'governance.blacklist.write', data, { timeoutMs });
    },

    async governanceWhitelistRead(
      requestId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(pluginId, requestId, 'governance.whitelist.read', {}, { timeoutMs });
    },

    async governanceWhitelistWrite(
      requestId: string,
      operation: 'set_enabled' | 'upsert' | 'delete',
      options: GovernanceWhitelistWriteOptions = {},
    ): Promise<Record<string, unknown>> {
      const { enabled, entryType, targetId, reason, timeoutMs = 30000 } = options;
      const data: Record<string, unknown> = { operation };
      if (operation === 'set_enabled') {
        if (enabled === undefined) {
          throw new Error('governanceWhitelistWrite set_enabled requires enabled');
        }
        data.enabled = enabled;
      } else if (operation === 'upsert') {
        if (!entryType || !targetId || !reason) {
          throw new Error('governanceWhitelistWrite upsert requires entryType, targetId, and reason');
        }
        data.entry_type = entryType;
        data.target_id = targetId;
        data.reason = reason;
      } else if (operation === 'delete') {
        if (!entryType || !targetId) {
          throw new Error('governanceWhitelistWrite delete requires entryType and targetId');
        }
        data.entry_type = entryType;
        data.target_id = targetId;
      } else {
        throw new Error('governanceWhitelistWrite requires operation set_enabled, upsert, or delete');
      }
      return await requestLocalAction(pluginId, requestId, 'governance.whitelist.write', data, { timeoutMs });
    },

    async governanceCommandPolicyRead(
      requestId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(pluginId, requestId, 'governance.command_policy.read', {}, { timeoutMs });
    },

    async schedulerCreate(
      requestId: string,
      taskId: string,
      cron: string,
      options: ActionOptions & { payload?: Record<string, unknown>; logLabel?: string } = {},
    ): Promise<Record<string, unknown>> {
      const { payload, logLabel, timeoutMs = 30000 } = options;
      const data: Record<string, unknown> = {
        task_id: taskId,
        cron,
        event_type: 'scheduler.trigger',
      };
      if (logLabel !== undefined) {
        data.log_label = logLabel;
      }
      if (payload !== undefined) {
        data.payload = payload;
      }
      return await requestLocalAction(
        pluginId,
        requestId,
        'scheduler.create',
        data,
        { timeoutMs },
      );
    },

    async exposeWebhook(
      requestId: string,
      route: string,
      options: ActionOptions & {
        methods?: string[];
        authStrategy?: string;
        header?: string;
        secretRef: string;
        signaturePrefix?: string;
        sourceIps?: string[];
        replayProtection?: {
          timestampHeader?: string;
          eventIdHeader?: string;
          toleranceSeconds?: number;
          enforce?: boolean;
        };
      } = { secretRef: '' },
    ): Promise<Record<string, unknown>> {
      const {
        methods = ['POST'],
        authStrategy = 'fixed_token',
        header = 'X-Webhook-Token',
        secretRef,
        signaturePrefix,
        sourceIps,
        replayProtection,
        timeoutMs = 30000,
      } = options;
      if (!secretRef) {
        throw new Error('exposeWebhook requires secretRef');
      }
      const data: Record<string, unknown> = {
        route,
        methods,
        auth_strategy: authStrategy,
        header,
        secret_ref: secretRef,
      };
      if (signaturePrefix !== undefined) {
        data.signature_prefix = signaturePrefix;
      }
      if (sourceIps !== undefined) {
        data.source_ips = sourceIps;
      }
      data.replay_protection = {
        timestamp_header: replayProtection?.timestampHeader ?? 'X-Raylea-Timestamp',
        event_id_header: replayProtection?.eventIdHeader ?? 'X-Raylea-Event-Id',
        tolerance_seconds: replayProtection?.toleranceSeconds ?? 300,
        enforce: replayProtection?.enforce ?? true,
      };
      return await requestLocalAction(
        pluginId,
        requestId,
        'event.expose_webhook',
        data,
        { timeoutMs },
      );
    },

    async renderImage(
      requestId: string,
      template: string,
      data: Record<string, unknown>,
      options: ActionOptions & {
        theme?: string;
        output?: string;
        fallbackText?: string;
      } = {},
    ): Promise<Record<string, unknown>> {
      const { theme, output, fallbackText, timeoutMs = 30000 } = options;
      const payload: Record<string, unknown> = { template, data };
      if (theme !== undefined) {
        payload.theme = theme;
      }
      if (output !== undefined) {
        payload.output = output;
      }
      if (fallbackText !== undefined) {
        payload.fallback_text = fallbackText;
      }
      return await requestLocalAction(
        pluginId,
        requestId,
        'render.image',
        payload,
        { timeoutMs },
      );
    },

    async pluginList(requestId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(pluginId, requestId, 'plugin.list', {}, { timeoutMs });
    },

    async onebotAction(
      requestId: string,
      action: string,
      data: Record<string, unknown> = {},
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestOneBotAction(requestId, action, data, options);
    },

    async providerAction(
      requestId: string,
      provider: ProviderName,
      action: string,
      data: Record<string, unknown> = {},
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedProviderAction(requestId, provider, action, data, options);
    },

    async messageGet(
      requestId: string,
      messageId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'message.get', { message_id: messageId }, options);
    },

    async messageDelete(
      requestId: string,
      messageId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'message.delete', { message_id: messageId }, options);
    },

    async messageHistoryGet(
      requestId: string,
      conversationType: ConversationType,
      conversationId: string,
      options: ActionOptions & { limit?: number } = {},
    ): Promise<Record<string, unknown>> {
      const { limit, timeoutMs = 30000 } = options;
      const data: Record<string, unknown> = {
        conversation_type: conversationType,
        conversation_id: conversationId,
      };
      if (limit !== undefined) {
        data.limit = limit;
      }
      return await requestOneBotAction(requestId, 'message.history.get', data, { timeoutMs });
    },

    async messageForwardGet(
      requestId: string,
      options: MessageForwardGetOptions = {},
    ): Promise<Record<string, unknown>> {
      const { messageId, forwardId, timeoutMs = 30000 } = options;
      if (!messageId && !forwardId) {
        throw new Error('messageForwardGet requires messageId or forwardId');
      }
      const data: Record<string, unknown> = {};
      if (messageId !== undefined) {
        data.message_id = messageId;
      }
      if (forwardId !== undefined) {
        data.forward_id = forwardId;
      }
      return await requestOneBotAction(requestId, 'message.forward.get', data, { timeoutMs });
    },

    async messageForwardSend(
      requestId: string,
      targetType: ConversationType,
      targetId: string,
      messages: Record<string, unknown>[],
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(
        requestId,
        'message.forward.send',
        { target_type: targetType, target_id: targetId, messages },
        options,
      );
    },

    async messageReadMark(
      requestId: string,
      options: MessageReadMarkOptions = {},
    ): Promise<Record<string, unknown>> {
      const { messageId, conversationType, conversationId, timeoutMs = 30000 } = options;
      if (messageId === undefined && (!conversationType || !conversationId)) {
        throw new Error('messageReadMark requires messageId or conversationType with conversationId');
      }
      const data: Record<string, unknown> = {};
      if (messageId !== undefined) {
        data.message_id = messageId;
      }
      if (conversationType !== undefined) {
        data.conversation_type = conversationType;
      }
      if (conversationId !== undefined) {
        data.conversation_id = conversationId;
      }
      return await requestOneBotAction(requestId, 'message.read.mark', data, { timeoutMs });
    },

    async friendRequestHandle(
      requestId: string,
      flag: string,
      approve: boolean,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'friend.request.handle', { flag, approve }, options);
    },

    async friendList(requestId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'friend.list', {}, options);
    },

    async friendRemarkSet(
      requestId: string,
      userId: string,
      remark: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(
        requestId,
        'friend.remark.set',
        { user_id: userId, remark },
        options,
      );
    },

    async userInfoGet(
      requestId: string,
      userId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'user.info.get', { user_id: userId }, options);
    },

    async userLikeSend(
      requestId: string,
      userId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'user.like.send', { user_id: userId }, options);
    },

    async groupList(requestId: string, options: ActionOptions = {}): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.list', {}, options);
    },

    async groupInfoGet(
      requestId: string,
      groupId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.info.get', { group_id: groupId }, options);
    },

    async groupMemberGet(
      requestId: string,
      groupId: string,
      userId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(
        requestId,
        'group.member.get',
        { group_id: groupId, user_id: userId },
        options,
      );
    },

    async groupMemberList(
      requestId: string,
      groupId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.member.list', { group_id: groupId }, options);
    },

    async groupRequestHandle(
      requestId: string,
      flag: string,
      approve: boolean,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.request.handle', { flag, approve }, options);
    },

    async groupLeave(
      requestId: string,
      groupId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.leave', { group_id: groupId }, options);
    },

    async groupAdminSet(
      requestId: string,
      groupId: string,
      userId: string,
      enabled: boolean,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(
        requestId,
        'group.admin.set',
        { group_id: groupId, user_id: userId, enabled },
        options,
      );
    },

    async groupBanSet(
      requestId: string,
      groupId: string,
      options: GroupBanSetOptions = {},
    ): Promise<Record<string, unknown>> {
      const { userId, durationSeconds, wholeGroup = false, timeoutMs = 30000 } = options;
      const data: Record<string, unknown> = { group_id: groupId, whole_group: wholeGroup };
      if (userId !== undefined) {
        data.user_id = userId;
      }
      if (durationSeconds !== undefined) {
        data.duration_seconds = durationSeconds;
      }
      return await requestOneBotAction(requestId, 'group.ban.set', data, { timeoutMs });
    },

    async groupCardSet(
      requestId: string,
      groupId: string,
      userId: string,
      card: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(
        requestId,
        'group.card.set',
        { group_id: groupId, user_id: userId, card },
        options,
      );
    },

    async groupTitleSet(
      requestId: string,
      groupId: string,
      userId: string,
      title: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(
        requestId,
        'group.title.set',
        { group_id: groupId, user_id: userId, title },
        options,
      );
    },

    async groupNameSet(
      requestId: string,
      groupId: string,
      name: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.name.set', { group_id: groupId, name }, options);
    },

    async groupAnnouncementList(
      requestId: string,
      groupId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.announcement.list', { group_id: groupId }, options);
    },

    async groupAnnouncementCreate(
      requestId: string,
      groupId: string,
      content: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.announcement.create', { group_id: groupId, content }, options);
    },

    async groupAnnouncementDelete(
      requestId: string,
      groupId: string,
      noticeId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(
        requestId,
        'group.announcement.delete',
        { group_id: groupId, notice_id: noticeId },
        options,
      );
    },

    async groupEssenceList(
      requestId: string,
      groupId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.essence.list', { group_id: groupId }, options);
    },

    async groupEssenceSet(
      requestId: string,
      messageId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.essence.set', { message_id: messageId }, options);
    },

    async groupEssenceUnset(
      requestId: string,
      messageId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.essence.unset', { message_id: messageId }, options);
    },

    async groupHonorGet(
      requestId: string,
      groupId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.honor.get', { group_id: groupId }, options);
    },

    async groupTodoSet(
      requestId: string,
      groupId: string,
      todo: Record<string, unknown>,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'group.todo.set', { group_id: groupId, todo }, options);
    },

    async fileGet(
      requestId: string,
      fileId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'file.get', { file_id: fileId }, options);
    },

    async fileDownload(
      requestId: string,
      fileId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'file.download', { file_id: fileId }, options);
    },

    async fileGroupUpload(
      requestId: string,
      groupId: string,
      fileName: string,
      fileUrl: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'file.group.upload', { group_id: groupId, file_name: fileName, file_url: fileUrl }, options);
    },

    async filePrivateUpload(
      requestId: string,
      userId: string,
      fileName: string,
      fileUrl: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'file.private.upload', { user_id: userId, file_name: fileName, file_url: fileUrl }, options);
    },

    async fileGroupUrlGet(
      requestId: string,
      groupId: string,
      fileId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'file.group.url.get', { group_id: groupId, file_id: fileId }, options);
    },

    async filePrivateUrlGet(
      requestId: string,
      userId: string,
      fileId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'file.private.url.get', { user_id: userId, file_id: fileId }, options);
    },

    async fileGroupFsInfo(
      requestId: string,
      groupId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'file.group.fs.info', { group_id: groupId }, options);
    },

    async fileGroupFsList(
      requestId: string,
      groupId: string,
      options: FileGroupFsListOptions = {},
    ): Promise<Record<string, unknown>> {
      const { folderId, timeoutMs = 30000 } = options;
      const data: Record<string, unknown> = { group_id: groupId };
      if (folderId !== undefined) {
        data.folder_id = folderId;
      }
      return await requestOneBotAction(requestId, 'file.group.fs.list', data, { timeoutMs });
    },

    async fileGroupFsMkdir(
      requestId: string,
      groupId: string,
      name: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'file.group.fs.mkdir', { group_id: groupId, name }, options);
    },

    async fileGroupFsDelete(
      requestId: string,
      groupId: string,
      options: FileGroupFsDeleteOptions = {},
    ): Promise<Record<string, unknown>> {
      const { folderId, fileId, timeoutMs = 30000 } = options;
      if (folderId === undefined && fileId === undefined) {
        throw new Error('fileGroupFsDelete requires folderId or fileId');
      }
      const data: Record<string, unknown> = { group_id: groupId };
      if (folderId !== undefined) {
        data.folder_id = folderId;
      }
      if (fileId !== undefined) {
        data.file_id = fileId;
      }
      return await requestOneBotAction(requestId, 'file.group.fs.delete', data, { timeoutMs });
    },

    async reactionSet(
      requestId: string,
      messageId: string,
      emoji: string,
      enabled = true,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'reaction.set', { message_id: messageId, emoji, enabled }, options);
    },

    async reactionList(
      requestId: string,
      messageId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'reaction.list', { message_id: messageId }, options);
    },

    async pokeSend(
      requestId: string,
      targetType: ConversationType,
      targetId: string,
      userId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedOneBotAction(requestId, 'poke.send', { target_type: targetType, target_id: targetId, user_id: userId }, options);
    },

    async napcatMessageEmojiLikeSet(
      requestId: string,
      messageId: string,
      emojiId: string,
      enabled = true,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedProviderAction(requestId, 'napcat', 'message_emoji.like.set', { message_id: messageId, emoji_id: emojiId, enabled }, options);
    },

    async napcatGroupSignSet(
      requestId: string,
      groupId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedProviderAction(requestId, 'napcat', 'group.sign.set', { group_id: groupId }, options);
    },

    async luckylilliaFriendGroupsGet(
      requestId: string,
      userId: string,
      options: ActionOptions = {},
    ): Promise<Record<string, unknown>> {
      return await requestNamedProviderAction(requestId, 'luckylillia', 'friend_groups.get', { user_id: userId }, options);
    },

    async run(): Promise<void> {
      for await (const frame of readFrames()) {
        const { type, plugin_id, request_id } = frame;

        if (type === 'init') {
          const initFrame = frame as InitFrame;
          pluginId = plugin_id;
          setBotId(initFrame.bot?.id ?? '');
          capabilities = Array.isArray(initFrame.capabilities)
            ? initFrame.capabilities.filter(
                (value): value is string => typeof value === 'string' && value.length > 0,
              )
            : [];
          superAdmins = Array.isArray(initFrame.permissions?.super_admins)
            ? initFrame.permissions.super_admins.filter(
                (value): value is string => typeof value === 'string' && value.length > 0,
              )
            : [];
          commandPrefixes = (initFrame.command_prefixes ?? []).filter(
            (value): value is string => typeof value === 'string' && value.length > 0,
          );
          if (commandPrefixes.length === 0) {
            commandPrefixes = ['/'];
          }
          sendInitAck(pluginId, request_id, subscriptions);
        } else if (type === 'event') {
          const eventFrame = frame as EventFrame;
          startEventHandler(eventFrame, plugin_id, request_id);
        } else if (type === 'ping') {
          sendPong(plugin_id || pluginId, request_id);
        } else if (type === 'shutdown') {
          break;
        }
      }
      await Promise.allSettled(Array.from(activeHandlers));
    },
  };

  function startEventHandler(frame: EventFrame, pid: string, requestId: string): void {
    let task: Promise<void>;
    task = handleEvent(frame, pid, requestId)
      .catch((error: unknown) => {
        sendError(pid, requestId, 'plugin.internal_error', formatErrorMessage(error));
      })
      .finally(() => {
        activeHandlers.delete(task);
      });
    activeHandlers.add(task);
  }

  async function handleEvent(
    frame: EventFrame,
    pid: string,
    requestId: string,
  ): Promise<void> {
    const event = frame.event ?? ({} as EventBody);
    updateBotIdentity(event);
    const command = event.payload?.command;

    if (command && commandHandlers.has(command)) {
      await invokeHandler(owner ?? plugin, commandHandlers.get(command)!, event, requestId);
      return;
    }

    for (const { type, handler } of eventHandlers) {
      if (type === null || type === event.event_type) {
        await invokeHandler(owner ?? plugin, handler, event, requestId);
        return;
      }
    }

    sendResult(pid, requestId, { handled: false });
  }

  function updateBotIdentity(event: EventBody): void {
    if (event.event_type !== 'bot.identity.changed') {
      return;
    }
    const targetId = event.target?.type === 'bot' ? event.target.id : undefined;
    const selfId = event.payload?.onebot?.self_id;
    const next = targetId || selfId || '';
    setBotId(next);
  }

  async function requestOneBotAction(
    requestId: string,
    action: string,
    data: Record<string, unknown>,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    const { timeoutMs = 30000 } = options;
    return await requestLocalAction(pluginId, requestId, action, data, { timeoutMs });
  }

  async function requestNamedOneBotAction(
    requestId: string,
    action: string,
    data: Record<string, unknown>,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return await requestOneBotAction(requestId, action, data, options);
  }

  async function requestNamedProviderAction(
    requestId: string,
    provider: ProviderName,
    action: string,
    data: Record<string, unknown>,
    options: ActionOptions = {},
  ): Promise<Record<string, unknown>> {
    return await requestOneBotAction(requestId, `provider.${provider}.${action}`, data, options);
  }

  return plugin;
}

export class RayleaBotPlugin {
  private readonly runtime: RayleaBotPluginRuntime;

  constructor() {
    this.runtime = createPluginRuntime(this);
  }

  get botId(): string {
    return this.runtime.botId;
  }

  awaitBotIdentity(timeoutMs?: number): Promise<string> {
    return this.runtime.awaitBotIdentity(timeoutMs);
  }

  get capabilities(): string[] {
    return this.runtime.capabilities;
  }

  get superAdmins(): string[] {
    return this.runtime.superAdmins;
  }

  get commandPrefixes(): string[] {
    return this.runtime.commandPrefixes;
  }

  get primaryCommandPrefix(): string {
    return this.runtime.primaryCommandPrefix;
  }

  onEvent(handler: EventHandler): this;
  onEvent(eventType: string, handler: EventHandler): this;
  onEvent(eventTypeOrHandler: string | EventHandler, handler?: EventHandler): this {
    if (typeof eventTypeOrHandler === 'function') {
      this.runtime.onEvent(bindHandler(this, eventTypeOrHandler));
      return this;
    }
    if (!handler) {
      throw new Error('onEvent requires a handler');
    }
    this.runtime.onEvent(eventTypeOrHandler, bindHandler(this, handler));
    return this;
  }

  onCommand(name: string, handler: EventHandler, aliases: string[] = []): this {
    this.runtime.onCommand(name, bindHandler(this, handler), aliases);
    return this;
  }

  subscribe(...eventTypes: string[]): this {
    this.runtime.subscribe(...eventTypes);
    return this;
  }
}

export interface RayleaBotPlugin extends Omit<
  RayleaBotPluginRuntime,
  'botId' | 'capabilities' | 'superAdmins' | 'commandPrefixes' | 'primaryCommandPrefix' | 'onEvent' | 'onCommand' | 'subscribe' | 'awaitBotIdentity'
> {}

const delegatedRuntimeMethods = [
  'sendMessage',
  'sendReply',
  'sendResult',
  'loggerWrite',
  'storageGet',
  'storageSet',
  'storageDelete',
  'storageList',
  'storageFileRead',
  'storageFileWrite',
  'storageFileDelete',
  'storageFileList',
  'httpRequest',
  'configRead',
  'configWrite',
  'governanceBlacklistRead',
  'governanceBlacklistWrite',
  'governanceWhitelistRead',
  'governanceWhitelistWrite',
  'governanceCommandPolicyRead',
  'schedulerCreate',
  'exposeWebhook',
  'renderImage',
  'pluginList',
  'onebotAction',
  'providerAction',
  'messageGet',
  'messageDelete',
  'messageHistoryGet',
  'messageForwardGet',
  'messageForwardSend',
  'messageReadMark',
  'friendRequestHandle',
  'friendList',
  'friendRemarkSet',
  'userInfoGet',
  'userLikeSend',
  'groupList',
  'groupInfoGet',
  'groupMemberGet',
  'groupMemberList',
  'groupRequestHandle',
  'groupLeave',
  'groupAdminSet',
  'groupBanSet',
  'groupCardSet',
  'groupTitleSet',
  'groupNameSet',
  'groupAnnouncementList',
  'groupAnnouncementCreate',
  'groupAnnouncementDelete',
  'groupEssenceList',
  'groupEssenceSet',
  'groupEssenceUnset',
  'groupHonorGet',
  'groupTodoSet',
  'fileGet',
  'fileDownload',
  'fileGroupUpload',
  'filePrivateUpload',
  'fileGroupUrlGet',
  'filePrivateUrlGet',
  'fileGroupFsInfo',
  'fileGroupFsList',
  'fileGroupFsMkdir',
  'fileGroupFsDelete',
  'reactionSet',
  'reactionList',
  'pokeSend',
  'napcatMessageEmojiLikeSet',
  'napcatGroupSignSet',
  'luckylilliaFriendGroupsGet',
  'run',
] as const;

for (const methodName of delegatedRuntimeMethods) {
  Object.defineProperty(RayleaBotPlugin.prototype, methodName, {
    value(this: RayleaBotPlugin, ...args: unknown[]) {
      const runtime = (this as unknown as { runtime: Record<string, (...innerArgs: unknown[]) => unknown> }).runtime;
      return runtime[methodName](...args);
    },
  });
}

export function createPlugin(): RayleaBotPlugin {
  return new RayleaBotPlugin();
}

function bindHandler(owner: RayleaBotPlugin, handler: EventHandler): EventHandler {
  return handler.bind(owner) as EventHandler;
}

async function invokeHandler(
  plugin: RayleaBotPluginRuntime,
  handler: EventHandler,
  event: EventBody,
  requestId: string,
): Promise<void> {
  if (handler.length >= 2) {
    await (handler as LegacyEventHandler)(event, requestId);
    return;
  }
  await (handler as ContextEventHandler)(new PluginEventContext(plugin, event, requestId));
}

function formatErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message || error.name;
  }
  return String(error);
}
