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
  sendReply(
    requestId: string,
    replyToEventId: string,
    segments: Segment[],
    options?: { fallbackToSendIfMissing?: boolean },
  ): void;

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
  schedulerCreate(
    requestId: string,
    taskId: string,
    cron: string,
    options?: ActionOptions & { payload?: Record<string, unknown> },
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

export function createPlugin(): RayleaBotPlugin {
  const eventHandlers: Array<{ type: string | null; handler: EventHandler }> = [];
  const commandHandlers = new Map<string, EventHandler>();
  const activeHandlers = new Set<Promise<void>>();
  let pluginId = '';
  let botId = '';
  let capabilities: string[] = [];
  let commandPrefixes = ['/'];
  let subscriptions: string[] | null = null;

  const plugin: RayleaBotPlugin = {
    get botId(): string {
      return botId;
    },

    get capabilities(): string[] {
      return [...capabilities];
    },

    get commandPrefixes(): string[] {
      return [...commandPrefixes];
    },

    get primaryCommandPrefix(): string {
      return commandPrefixes[0] || '/';
    },

    onEvent(eventTypeOrHandler: string | EventHandler, handler?: EventHandler): RayleaBotPlugin {
      if (typeof eventTypeOrHandler === 'function') {
        eventHandlers.push({ type: null, handler: eventTypeOrHandler });
      } else {
        eventHandlers.push({ type: eventTypeOrHandler, handler: handler! });
      }
      return plugin;
    },

    onCommand(name: string, handler: EventHandler, aliases: string[] = []): RayleaBotPlugin {
      commandHandlers.set(name, handler);
      for (const alias of aliases) {
        commandHandlers.set(alias, handler);
      }
      return plugin;
    },

    subscribe(...eventTypes: string[]): RayleaBotPlugin {
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

    async schedulerCreate(
      requestId: string,
      taskId: string,
      cron: string,
      options: ActionOptions & { payload?: Record<string, unknown> } = {},
    ): Promise<Record<string, unknown>> {
      const { payload, timeoutMs = 30000 } = options;
      const data: Record<string, unknown> = {
        task_id: taskId,
        cron,
        event_type: 'scheduler.trigger',
      };
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
      } = { secretRef: '' },
    ): Promise<Record<string, unknown>> {
      const {
        methods = ['POST'],
        authStrategy = 'fixed_token',
        header = 'X-Webhook-Token',
        secretRef,
        signaturePrefix,
        sourceIps,
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
          botId = initFrame.bot?.id ?? '';
          capabilities = Array.isArray(initFrame.capabilities)
            ? initFrame.capabilities.filter(
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
    const command = event.payload?.command;

    if (command && commandHandlers.has(command)) {
      await commandHandlers.get(command)!(event, requestId);
      return;
    }

    for (const { type, handler } of eventHandlers) {
      if (type === null || type === event.event_type) {
        await handler(event, requestId);
        return;
      }
    }

    sendResult(pid, requestId, { handled: false });
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

function formatErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message || error.name;
  }
  return String(error);
}
