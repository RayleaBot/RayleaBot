import { readFrames, requestLocalAction, sendAction, sendError, sendInitAck, sendPong, sendResult, } from './protocol.js';
export { textSegment, imageSegment, atSegment, atAllSegment, faceSegment, replySegment, passthroughSegment, recordSegment, videoSegment, markdownSegment, fileSegment, flashFileSegment, jsonSegment, xmlSegment, musicSegment, contactSegment, forwardSegment, nodeSegment, pokeSegment, diceSegment, rpsSegment, mfaceSegment, keyboardSegment, shakeSegment, } from './types.js';
export { ActionError } from './protocol.js';
export class PluginEventContext {
    event;
    requestId;
    plugin;
    constructor(plugin, event, requestId) {
        this.plugin = plugin;
        this.event = event;
        this.requestId = requestId;
    }
    get payload() {
        return this.event.payload ?? {};
    }
    get target() {
        return this.event.target;
    }
    get actor() {
        return this.event.actor;
    }
    get message() {
        return this.event.message ?? {};
    }
    get eventType() {
        return this.event.event_type;
    }
    get command() {
        return this.event.payload?.command;
    }
    get args() {
        return this.event.payload?.args ?? [];
    }
    get plainText() {
        return this.event.message?.plain_text ?? '';
    }
    get targetType() {
        return this.event.target?.type ?? 'group';
    }
    get targetId() {
        return this.event.target?.id ?? '';
    }
    get botId() {
        return this.plugin.botId;
    }
    awaitBotIdentity(timeoutMs) {
        return this.plugin.awaitBotIdentity(timeoutMs);
    }
    get capabilities() {
        return this.plugin.capabilities;
    }
    get superAdmins() {
        return this.plugin.superAdmins;
    }
    get commandPrefixes() {
        return this.plugin.commandPrefixes;
    }
    get primaryCommandPrefix() {
        return this.plugin.primaryCommandPrefix;
    }
    sendMessage(segments, options = {}) {
        this.plugin.sendMessage(this.requestId, options.targetType ?? this.targetType, options.targetId ?? this.targetId, segments);
    }
    sendText(text, options = {}) {
        this.sendMessage([{ type: 'text', data: { text } }], options);
    }
    sendReply(replyToEventId, segments, options = {}) {
        this.plugin.sendReply(this.requestId, replyToEventId, segments, options);
    }
    sendResult(data = {}) {
        this.plugin.sendResult(this.requestId, data);
    }
    loggerWrite(level, message, fields, options = {}) {
        return this.plugin.loggerWrite(this.requestId, level, message, fields, options);
    }
    storageGet(key, options = {}) {
        return this.plugin.storageGet(this.requestId, key, options);
    }
    storageSet(key, value, options = {}) {
        return this.plugin.storageSet(this.requestId, key, value, options);
    }
    storageDelete(key, options = {}) {
        return this.plugin.storageDelete(this.requestId, key, options);
    }
    storageList(prefix = '', options = {}) {
        return this.plugin.storageList(this.requestId, prefix, options);
    }
    storageFileRead(path, options = {}) {
        return this.plugin.storageFileRead(this.requestId, path, options);
    }
    storageFileWrite(path, options = {}) {
        return this.plugin.storageFileWrite(this.requestId, path, options);
    }
    storageFileDelete(path, options = {}) {
        return this.plugin.storageFileDelete(this.requestId, path, options);
    }
    storageFileList(prefix = '', options = {}) {
        return this.plugin.storageFileList(this.requestId, prefix, options);
    }
    httpRequest(method, url, options = {}) {
        return this.plugin.httpRequest(this.requestId, method, url, options);
    }
    configRead(keys, options = {}) {
        return this.plugin.configRead(this.requestId, keys, options);
    }
    configWrite(values, options = {}) {
        return this.plugin.configWrite(this.requestId, values, options);
    }
    governanceBlacklistRead(options = {}) {
        return this.plugin.governanceBlacklistRead(this.requestId, options);
    }
    governanceBlacklistWrite(operation, options = {}) {
        return this.plugin.governanceBlacklistWrite(this.requestId, operation, options);
    }
    governanceWhitelistRead(options = {}) {
        return this.plugin.governanceWhitelistRead(this.requestId, options);
    }
    governanceWhitelistWrite(operation, options = {}) {
        return this.plugin.governanceWhitelistWrite(this.requestId, operation, options);
    }
    governanceCommandPolicyRead(options = {}) {
        return this.plugin.governanceCommandPolicyRead(this.requestId, options);
    }
    schedulerCreate(taskId, cron, options = {}) {
        return this.plugin.schedulerCreate(this.requestId, taskId, cron, options);
    }
    exposeWebhook(route, options) {
        return this.plugin.exposeWebhook(this.requestId, route, options);
    }
    renderImage(template, data, options = {}) {
        return this.plugin.renderImage(this.requestId, template, data, options);
    }
    pluginList(options = {}) {
        return this.plugin.pluginList(this.requestId, options);
    }
    messageGet(messageId, options = {}) {
        return this.plugin.messageGet(this.requestId, messageId, options);
    }
    messageDelete(messageId, options = {}) {
        return this.plugin.messageDelete(this.requestId, messageId, options);
    }
    messageHistoryGet(conversationType, conversationId, options = {}) {
        return this.plugin.messageHistoryGet(this.requestId, conversationType, conversationId, options);
    }
    messageForwardGet(options = {}) {
        return this.plugin.messageForwardGet(this.requestId, options);
    }
    messageForwardSend(targetType, targetId, messages, options = {}) {
        return this.plugin.messageForwardSend(this.requestId, targetType, targetId, messages, options);
    }
    messageReadMark(options = {}) {
        return this.plugin.messageReadMark(this.requestId, options);
    }
    friendRequestHandle(flag, approve, options = {}) {
        return this.plugin.friendRequestHandle(this.requestId, flag, approve, options);
    }
    friendList(options = {}) {
        return this.plugin.friendList(this.requestId, options);
    }
    friendRemarkSet(userId, remark, options = {}) {
        return this.plugin.friendRemarkSet(this.requestId, userId, remark, options);
    }
    userInfoGet(userId, options = {}) {
        return this.plugin.userInfoGet(this.requestId, userId, options);
    }
    userLikeSend(userId, options = {}) {
        return this.plugin.userLikeSend(this.requestId, userId, options);
    }
    groupList(options = {}) {
        return this.plugin.groupList(this.requestId, options);
    }
    groupInfoGet(groupId, options = {}) {
        return this.plugin.groupInfoGet(this.requestId, groupId, options);
    }
    groupMemberGet(groupId, userId, options = {}) {
        return this.plugin.groupMemberGet(this.requestId, groupId, userId, options);
    }
    groupMemberList(groupId, options = {}) {
        return this.plugin.groupMemberList(this.requestId, groupId, options);
    }
    groupRequestHandle(flag, approve, options = {}) {
        return this.plugin.groupRequestHandle(this.requestId, flag, approve, options);
    }
    groupLeave(groupId, options = {}) {
        return this.plugin.groupLeave(this.requestId, groupId, options);
    }
    groupAdminSet(groupId, userId, enabled, options = {}) {
        return this.plugin.groupAdminSet(this.requestId, groupId, userId, enabled, options);
    }
    groupBanSet(groupId, options = {}) {
        return this.plugin.groupBanSet(this.requestId, groupId, options);
    }
    groupCardSet(groupId, userId, card, options = {}) {
        return this.plugin.groupCardSet(this.requestId, groupId, userId, card, options);
    }
    groupTitleSet(groupId, userId, title, options = {}) {
        return this.plugin.groupTitleSet(this.requestId, groupId, userId, title, options);
    }
    groupNameSet(groupId, name, options = {}) {
        return this.plugin.groupNameSet(this.requestId, groupId, name, options);
    }
    groupAnnouncementList(groupId, options = {}) {
        return this.plugin.groupAnnouncementList(this.requestId, groupId, options);
    }
    groupAnnouncementCreate(groupId, content, options = {}) {
        return this.plugin.groupAnnouncementCreate(this.requestId, groupId, content, options);
    }
    groupAnnouncementDelete(groupId, noticeId, options = {}) {
        return this.plugin.groupAnnouncementDelete(this.requestId, groupId, noticeId, options);
    }
    groupEssenceList(groupId, options = {}) {
        return this.plugin.groupEssenceList(this.requestId, groupId, options);
    }
    groupEssenceSet(messageId, options = {}) {
        return this.plugin.groupEssenceSet(this.requestId, messageId, options);
    }
    groupEssenceUnset(messageId, options = {}) {
        return this.plugin.groupEssenceUnset(this.requestId, messageId, options);
    }
    groupHonorGet(groupId, options = {}) {
        return this.plugin.groupHonorGet(this.requestId, groupId, options);
    }
    groupTodoSet(groupId, todo, options = {}) {
        return this.plugin.groupTodoSet(this.requestId, groupId, todo, options);
    }
    fileGet(fileId, options = {}) {
        return this.plugin.fileGet(this.requestId, fileId, options);
    }
    fileDownload(fileId, options = {}) {
        return this.plugin.fileDownload(this.requestId, fileId, options);
    }
    fileGroupUpload(groupId, fileName, fileUrl, options = {}) {
        return this.plugin.fileGroupUpload(this.requestId, groupId, fileName, fileUrl, options);
    }
    filePrivateUpload(userId, fileName, fileUrl, options = {}) {
        return this.plugin.filePrivateUpload(this.requestId, userId, fileName, fileUrl, options);
    }
    fileGroupUrlGet(groupId, fileId, options = {}) {
        return this.plugin.fileGroupUrlGet(this.requestId, groupId, fileId, options);
    }
    filePrivateUrlGet(userId, fileId, options = {}) {
        return this.plugin.filePrivateUrlGet(this.requestId, userId, fileId, options);
    }
    fileGroupFsInfo(groupId, options = {}) {
        return this.plugin.fileGroupFsInfo(this.requestId, groupId, options);
    }
    fileGroupFsList(groupId, options = {}) {
        return this.plugin.fileGroupFsList(this.requestId, groupId, options);
    }
    fileGroupFsMkdir(groupId, name, options = {}) {
        return this.plugin.fileGroupFsMkdir(this.requestId, groupId, name, options);
    }
    fileGroupFsDelete(groupId, options = {}) {
        return this.plugin.fileGroupFsDelete(this.requestId, groupId, options);
    }
    reactionSet(messageId, emoji, enabled = true, options = {}) {
        return this.plugin.reactionSet(this.requestId, messageId, emoji, enabled, options);
    }
    reactionList(messageId, options = {}) {
        return this.plugin.reactionList(this.requestId, messageId, options);
    }
    pokeSend(targetType, targetId, userId, options = {}) {
        return this.plugin.pokeSend(this.requestId, targetType, targetId, userId, options);
    }
    napcatMessageEmojiLikeSet(messageId, emojiId, enabled = true, options = {}) {
        return this.plugin.napcatMessageEmojiLikeSet(this.requestId, messageId, emojiId, enabled, options);
    }
    napcatGroupSignSet(groupId, options = {}) {
        return this.plugin.napcatGroupSignSet(this.requestId, groupId, options);
    }
    luckylilliaFriendGroupsGet(userId, options = {}) {
        return this.plugin.luckylilliaFriendGroupsGet(this.requestId, userId, options);
    }
    onebotAction(action, data = {}, options = {}) {
        return this.plugin.onebotAction(this.requestId, action, data, options);
    }
    providerAction(provider, action, data = {}, options = {}) {
        return this.plugin.providerAction(this.requestId, provider, action, data, options);
    }
}
function createPluginRuntime(owner) {
    const eventHandlers = [];
    const commandHandlers = new Map();
    const activeHandlers = new Set();
    let pluginId = '';
    let botId = '';
    let capabilities = [];
    let superAdmins = [];
    let commandPrefixes = ['/'];
    let subscriptions = null;
    const botIdentityWaiters = new Set();
    function setBotId(next) {
        botId = next || '';
        if (botId) {
            const current = botId;
            const pending = Array.from(botIdentityWaiters);
            botIdentityWaiters.clear();
            for (const resolve of pending) {
                try {
                    resolve(current);
                }
                catch {
                    // resolver throwing is the caller's bug; swallowing keeps the
                    // event loop healthy for other waiters.
                }
            }
        }
    }
    function awaitBotIdentityImpl(timeoutMs) {
        if (botId) {
            return Promise.resolve(botId);
        }
        const clampedTimeout = Math.max(0, Math.floor(timeoutMs));
        return new Promise((resolve) => {
            let settled = false;
            let timer;
            const wrappedResolve = (value) => finish(value);
            const finish = (value) => {
                if (settled)
                    return;
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
                if (typeof timer === 'object' && timer && 'unref' in timer && typeof timer.unref === 'function') {
                    timer.unref();
                }
            }
            else {
                // Zero timeout: keep waiting indefinitely.
            }
        });
    }
    const plugin = {
        get botId() {
            return botId;
        },
        awaitBotIdentity(timeoutMs = 30_000) {
            return awaitBotIdentityImpl(timeoutMs);
        },
        get capabilities() {
            return [...capabilities];
        },
        get superAdmins() {
            return [...superAdmins];
        },
        get commandPrefixes() {
            return [...commandPrefixes];
        },
        get primaryCommandPrefix() {
            return commandPrefixes[0] || '/';
        },
        onEvent(eventTypeOrHandler, handler) {
            if (typeof eventTypeOrHandler === 'function') {
                eventHandlers.push({ type: null, handler: eventTypeOrHandler });
            }
            else {
                eventHandlers.push({ type: eventTypeOrHandler, handler: handler });
            }
            return plugin;
        },
        onCommand(name, handler, aliases = []) {
            commandHandlers.set(name, handler);
            for (const alias of aliases) {
                commandHandlers.set(alias, handler);
            }
            return plugin;
        },
        subscribe(...eventTypes) {
            subscriptions = eventTypes;
            return plugin;
        },
        sendMessage(requestId, targetType, targetId, segments) {
            sendAction(pluginId, requestId, 'message.send', {
                target_type: targetType,
                target_id: targetId,
                message: { segments },
            });
        },
        sendReply(requestId, replyToEventId, segments, options = {}) {
            const data = {
                reply_to_event_id: replyToEventId,
                message: { segments },
            };
            if (options.fallbackToSendIfMissing) {
                data.fallback_to_send_if_missing = true;
            }
            sendAction(pluginId, requestId, 'message.reply', data);
        },
        sendResult(requestId, data = {}) {
            sendResult(pluginId, requestId, data);
        },
        async loggerWrite(requestId, level, message, fields, options = {}) {
            const data = { level, message };
            if (fields !== undefined) {
                data.fields = fields;
            }
            return await requestLocalAction(pluginId, requestId, 'logger.write', data, options);
        },
        async storageGet(requestId, key, options = {}) {
            return await requestLocalAction(pluginId, requestId, 'storage.kv', { operation: 'get', key }, options);
        },
        async storageSet(requestId, key, value, options = {}) {
            return await requestLocalAction(pluginId, requestId, 'storage.kv', { operation: 'set', key, value }, options);
        },
        async storageDelete(requestId, key, options = {}) {
            return await requestLocalAction(pluginId, requestId, 'storage.kv', { operation: 'delete', key }, options);
        },
        async storageList(requestId, prefix = '', options = {}) {
            return await requestLocalAction(pluginId, requestId, 'storage.kv', { operation: 'list', prefix }, options);
        },
        async storageFileRead(requestId, path, options = {}) {
            const root = options.root ?? 'plugin_data';
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'storage.file', { operation: 'read', root, path }, { timeoutMs });
        },
        async storageFileWrite(requestId, path, options = {}) {
            const { root = 'plugin_data', contentText, contentBase64, timeoutMs = 30000 } = options;
            if ((contentText === undefined) === (contentBase64 === undefined)) {
                throw new Error('storageFileWrite requires exactly one of contentText or contentBase64');
            }
            const data = { operation: 'write', root, path };
            if (contentText !== undefined) {
                data.content_text = contentText;
            }
            else {
                data.content_base64 = contentBase64;
            }
            return await requestLocalAction(pluginId, requestId, 'storage.file', data, { timeoutMs });
        },
        async storageFileDelete(requestId, path, options = {}) {
            const root = options.root ?? 'plugin_data';
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'storage.file', { operation: 'delete', root, path }, { timeoutMs });
        },
        async storageFileList(requestId, prefix = '', options = {}) {
            const root = options.root ?? 'plugin_data';
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'storage.file', { operation: 'list', root, prefix }, { timeoutMs });
        },
        async httpRequest(requestId, method, url, options = {}) {
            const { headers, timeoutMs = 30000, timeoutSeconds, bodyText, bodyBase64 } = options;
            if (bodyText !== undefined && bodyBase64 !== undefined) {
                throw new Error('httpRequest requires at most one of bodyText or bodyBase64');
            }
            const data = { method, url };
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
        async configRead(requestId, keys, options = {}) {
            if (!Array.isArray(keys) || keys.length === 0) {
                throw new Error('configRead requires at least one key');
            }
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'config.read', { keys }, { timeoutMs });
        },
        async configWrite(requestId, values, options = {}) {
            if (!values || typeof values !== 'object' || Object.keys(values).length === 0) {
                throw new Error('configWrite requires at least one key/value pair');
            }
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'config.write', { values }, { timeoutMs });
        },
        async governanceBlacklistRead(requestId, options = {}) {
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'governance.blacklist.read', {}, { timeoutMs });
        },
        async governanceBlacklistWrite(requestId, operation, options = {}) {
            const { entryType, targetId, reason, timeoutMs = 30000 } = options;
            const data = { operation };
            if (operation === 'upsert') {
                if (!entryType || !targetId || !reason) {
                    throw new Error('governanceBlacklistWrite upsert requires entryType, targetId, and reason');
                }
                data.entry_type = entryType;
                data.target_id = targetId;
                data.reason = reason;
            }
            else if (operation === 'delete') {
                if (!entryType || !targetId) {
                    throw new Error('governanceBlacklistWrite delete requires entryType and targetId');
                }
                data.entry_type = entryType;
                data.target_id = targetId;
            }
            else {
                throw new Error('governanceBlacklistWrite requires operation upsert or delete');
            }
            return await requestLocalAction(pluginId, requestId, 'governance.blacklist.write', data, { timeoutMs });
        },
        async governanceWhitelistRead(requestId, options = {}) {
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'governance.whitelist.read', {}, { timeoutMs });
        },
        async governanceWhitelistWrite(requestId, operation, options = {}) {
            const { enabled, entryType, targetId, reason, timeoutMs = 30000 } = options;
            const data = { operation };
            if (operation === 'set_enabled') {
                if (enabled === undefined) {
                    throw new Error('governanceWhitelistWrite set_enabled requires enabled');
                }
                data.enabled = enabled;
            }
            else if (operation === 'upsert') {
                if (!entryType || !targetId || !reason) {
                    throw new Error('governanceWhitelistWrite upsert requires entryType, targetId, and reason');
                }
                data.entry_type = entryType;
                data.target_id = targetId;
                data.reason = reason;
            }
            else if (operation === 'delete') {
                if (!entryType || !targetId) {
                    throw new Error('governanceWhitelistWrite delete requires entryType and targetId');
                }
                data.entry_type = entryType;
                data.target_id = targetId;
            }
            else {
                throw new Error('governanceWhitelistWrite requires operation set_enabled, upsert, or delete');
            }
            return await requestLocalAction(pluginId, requestId, 'governance.whitelist.write', data, { timeoutMs });
        },
        async governanceCommandPolicyRead(requestId, options = {}) {
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'governance.command_policy.read', {}, { timeoutMs });
        },
        async schedulerCreate(requestId, taskId, cron, options = {}) {
            const { payload, logLabel, timeoutMs = 30000 } = options;
            const data = {
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
            return await requestLocalAction(pluginId, requestId, 'scheduler.create', data, { timeoutMs });
        },
        async exposeWebhook(requestId, route, options = { secretRef: '' }) {
            const { methods = ['POST'], authStrategy = 'fixed_token', header = 'X-Webhook-Token', secretRef, signaturePrefix, sourceIps, replayProtection, timeoutMs = 30000, } = options;
            if (!secretRef) {
                throw new Error('exposeWebhook requires secretRef');
            }
            const data = {
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
            return await requestLocalAction(pluginId, requestId, 'event.expose_webhook', data, { timeoutMs });
        },
        async renderImage(requestId, template, data, options = {}) {
            const { theme, output, fallbackText, timeoutMs = 30000 } = options;
            const payload = { template, data };
            if (theme !== undefined) {
                payload.theme = theme;
            }
            if (output !== undefined) {
                payload.output = output;
            }
            if (fallbackText !== undefined) {
                payload.fallback_text = fallbackText;
            }
            return await requestLocalAction(pluginId, requestId, 'render.image', payload, { timeoutMs });
        },
        async pluginList(requestId, options = {}) {
            const { timeoutMs = 30000 } = options;
            return await requestLocalAction(pluginId, requestId, 'plugin.list', {}, { timeoutMs });
        },
        async onebotAction(requestId, action, data = {}, options = {}) {
            return await requestOneBotAction(requestId, action, data, options);
        },
        async providerAction(requestId, provider, action, data = {}, options = {}) {
            return await requestNamedProviderAction(requestId, provider, action, data, options);
        },
        async messageGet(requestId, messageId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'message.get', { message_id: messageId }, options);
        },
        async messageDelete(requestId, messageId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'message.delete', { message_id: messageId }, options);
        },
        async messageHistoryGet(requestId, conversationType, conversationId, options = {}) {
            const { limit, timeoutMs = 30000 } = options;
            const data = {
                conversation_type: conversationType,
                conversation_id: conversationId,
            };
            if (limit !== undefined) {
                data.limit = limit;
            }
            return await requestOneBotAction(requestId, 'message.history.get', data, { timeoutMs });
        },
        async messageForwardGet(requestId, options = {}) {
            const { messageId, forwardId, timeoutMs = 30000 } = options;
            if (!messageId && !forwardId) {
                throw new Error('messageForwardGet requires messageId or forwardId');
            }
            const data = {};
            if (messageId !== undefined) {
                data.message_id = messageId;
            }
            if (forwardId !== undefined) {
                data.forward_id = forwardId;
            }
            return await requestOneBotAction(requestId, 'message.forward.get', data, { timeoutMs });
        },
        async messageForwardSend(requestId, targetType, targetId, messages, options = {}) {
            return await requestNamedOneBotAction(requestId, 'message.forward.send', { target_type: targetType, target_id: targetId, messages }, options);
        },
        async messageReadMark(requestId, options = {}) {
            const { messageId, conversationType, conversationId, timeoutMs = 30000 } = options;
            if (messageId === undefined && (!conversationType || !conversationId)) {
                throw new Error('messageReadMark requires messageId or conversationType with conversationId');
            }
            const data = {};
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
        async friendRequestHandle(requestId, flag, approve, options = {}) {
            return await requestNamedOneBotAction(requestId, 'friend.request.handle', { flag, approve }, options);
        },
        async friendList(requestId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'friend.list', {}, options);
        },
        async friendRemarkSet(requestId, userId, remark, options = {}) {
            return await requestNamedOneBotAction(requestId, 'friend.remark.set', { user_id: userId, remark }, options);
        },
        async userInfoGet(requestId, userId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'user.info.get', { user_id: userId }, options);
        },
        async userLikeSend(requestId, userId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'user.like.send', { user_id: userId }, options);
        },
        async groupList(requestId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.list', {}, options);
        },
        async groupInfoGet(requestId, groupId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.info.get', { group_id: groupId }, options);
        },
        async groupMemberGet(requestId, groupId, userId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.member.get', { group_id: groupId, user_id: userId }, options);
        },
        async groupMemberList(requestId, groupId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.member.list', { group_id: groupId }, options);
        },
        async groupRequestHandle(requestId, flag, approve, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.request.handle', { flag, approve }, options);
        },
        async groupLeave(requestId, groupId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.leave', { group_id: groupId }, options);
        },
        async groupAdminSet(requestId, groupId, userId, enabled, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.admin.set', { group_id: groupId, user_id: userId, enabled }, options);
        },
        async groupBanSet(requestId, groupId, options = {}) {
            const { userId, durationSeconds, wholeGroup = false, timeoutMs = 30000 } = options;
            const data = { group_id: groupId, whole_group: wholeGroup };
            if (userId !== undefined) {
                data.user_id = userId;
            }
            if (durationSeconds !== undefined) {
                data.duration_seconds = durationSeconds;
            }
            return await requestOneBotAction(requestId, 'group.ban.set', data, { timeoutMs });
        },
        async groupCardSet(requestId, groupId, userId, card, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.card.set', { group_id: groupId, user_id: userId, card }, options);
        },
        async groupTitleSet(requestId, groupId, userId, title, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.title.set', { group_id: groupId, user_id: userId, title }, options);
        },
        async groupNameSet(requestId, groupId, name, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.name.set', { group_id: groupId, name }, options);
        },
        async groupAnnouncementList(requestId, groupId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.announcement.list', { group_id: groupId }, options);
        },
        async groupAnnouncementCreate(requestId, groupId, content, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.announcement.create', { group_id: groupId, content }, options);
        },
        async groupAnnouncementDelete(requestId, groupId, noticeId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.announcement.delete', { group_id: groupId, notice_id: noticeId }, options);
        },
        async groupEssenceList(requestId, groupId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.essence.list', { group_id: groupId }, options);
        },
        async groupEssenceSet(requestId, messageId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.essence.set', { message_id: messageId }, options);
        },
        async groupEssenceUnset(requestId, messageId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.essence.unset', { message_id: messageId }, options);
        },
        async groupHonorGet(requestId, groupId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.honor.get', { group_id: groupId }, options);
        },
        async groupTodoSet(requestId, groupId, todo, options = {}) {
            return await requestNamedOneBotAction(requestId, 'group.todo.set', { group_id: groupId, todo }, options);
        },
        async fileGet(requestId, fileId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'file.get', { file_id: fileId }, options);
        },
        async fileDownload(requestId, fileId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'file.download', { file_id: fileId }, options);
        },
        async fileGroupUpload(requestId, groupId, fileName, fileUrl, options = {}) {
            return await requestNamedOneBotAction(requestId, 'file.group.upload', { group_id: groupId, file_name: fileName, file_url: fileUrl }, options);
        },
        async filePrivateUpload(requestId, userId, fileName, fileUrl, options = {}) {
            return await requestNamedOneBotAction(requestId, 'file.private.upload', { user_id: userId, file_name: fileName, file_url: fileUrl }, options);
        },
        async fileGroupUrlGet(requestId, groupId, fileId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'file.group.url.get', { group_id: groupId, file_id: fileId }, options);
        },
        async filePrivateUrlGet(requestId, userId, fileId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'file.private.url.get', { user_id: userId, file_id: fileId }, options);
        },
        async fileGroupFsInfo(requestId, groupId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'file.group.fs.info', { group_id: groupId }, options);
        },
        async fileGroupFsList(requestId, groupId, options = {}) {
            const { folderId, timeoutMs = 30000 } = options;
            const data = { group_id: groupId };
            if (folderId !== undefined) {
                data.folder_id = folderId;
            }
            return await requestOneBotAction(requestId, 'file.group.fs.list', data, { timeoutMs });
        },
        async fileGroupFsMkdir(requestId, groupId, name, options = {}) {
            return await requestNamedOneBotAction(requestId, 'file.group.fs.mkdir', { group_id: groupId, name }, options);
        },
        async fileGroupFsDelete(requestId, groupId, options = {}) {
            const { folderId, fileId, timeoutMs = 30000 } = options;
            if (folderId === undefined && fileId === undefined) {
                throw new Error('fileGroupFsDelete requires folderId or fileId');
            }
            const data = { group_id: groupId };
            if (folderId !== undefined) {
                data.folder_id = folderId;
            }
            if (fileId !== undefined) {
                data.file_id = fileId;
            }
            return await requestOneBotAction(requestId, 'file.group.fs.delete', data, { timeoutMs });
        },
        async reactionSet(requestId, messageId, emoji, enabled = true, options = {}) {
            return await requestNamedOneBotAction(requestId, 'reaction.set', { message_id: messageId, emoji, enabled }, options);
        },
        async reactionList(requestId, messageId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'reaction.list', { message_id: messageId }, options);
        },
        async pokeSend(requestId, targetType, targetId, userId, options = {}) {
            return await requestNamedOneBotAction(requestId, 'poke.send', { target_type: targetType, target_id: targetId, user_id: userId }, options);
        },
        async napcatMessageEmojiLikeSet(requestId, messageId, emojiId, enabled = true, options = {}) {
            return await requestNamedProviderAction(requestId, 'napcat', 'message_emoji.like.set', { message_id: messageId, emoji_id: emojiId, enabled }, options);
        },
        async napcatGroupSignSet(requestId, groupId, options = {}) {
            return await requestNamedProviderAction(requestId, 'napcat', 'group.sign.set', { group_id: groupId }, options);
        },
        async luckylilliaFriendGroupsGet(requestId, userId, options = {}) {
            return await requestNamedProviderAction(requestId, 'luckylillia', 'friend_groups.get', { user_id: userId }, options);
        },
        async run() {
            for await (const frame of readFrames()) {
                const { type, plugin_id, request_id } = frame;
                if (type === 'init') {
                    const initFrame = frame;
                    pluginId = plugin_id;
                    setBotId(initFrame.bot?.id ?? '');
                    capabilities = Array.isArray(initFrame.capabilities)
                        ? initFrame.capabilities.filter((value) => typeof value === 'string' && value.length > 0)
                        : [];
                    superAdmins = Array.isArray(initFrame.permissions?.super_admins)
                        ? initFrame.permissions.super_admins.filter((value) => typeof value === 'string' && value.length > 0)
                        : [];
                    commandPrefixes = (initFrame.command_prefixes ?? []).filter((value) => typeof value === 'string' && value.length > 0);
                    if (commandPrefixes.length === 0) {
                        commandPrefixes = ['/'];
                    }
                    sendInitAck(pluginId, request_id, subscriptions);
                }
                else if (type === 'event') {
                    const eventFrame = frame;
                    startEventHandler(eventFrame, plugin_id, request_id);
                }
                else if (type === 'ping') {
                    sendPong(plugin_id || pluginId, request_id);
                }
                else if (type === 'shutdown') {
                    break;
                }
            }
            await Promise.allSettled(Array.from(activeHandlers));
        },
    };
    function startEventHandler(frame, pid, requestId) {
        let task;
        task = handleEvent(frame, pid, requestId)
            .catch((error) => {
            sendError(pid, requestId, 'plugin.internal_error', formatErrorMessage(error));
        })
            .finally(() => {
            activeHandlers.delete(task);
        });
        activeHandlers.add(task);
    }
    async function handleEvent(frame, pid, requestId) {
        const event = frame.event ?? {};
        updateBotIdentity(event);
        const command = event.payload?.command;
        if (command && commandHandlers.has(command)) {
            await invokeHandler(owner ?? plugin, commandHandlers.get(command), event, requestId);
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
    function updateBotIdentity(event) {
        if (event.event_type !== 'bot.identity.changed') {
            return;
        }
        const targetId = event.target?.type === 'bot' ? event.target.id : undefined;
        const selfId = event.payload?.onebot?.self_id;
        const next = targetId || selfId || '';
        setBotId(next);
    }
    async function requestOneBotAction(requestId, action, data, options = {}) {
        const { timeoutMs = 30000 } = options;
        return await requestLocalAction(pluginId, requestId, action, data, { timeoutMs });
    }
    async function requestNamedOneBotAction(requestId, action, data, options = {}) {
        return await requestOneBotAction(requestId, action, data, options);
    }
    async function requestNamedProviderAction(requestId, provider, action, data, options = {}) {
        return await requestOneBotAction(requestId, `provider.${provider}.${action}`, data, options);
    }
    return plugin;
}
export class RayleaBotPlugin {
    runtime;
    constructor() {
        this.runtime = createPluginRuntime(this);
    }
    get botId() {
        return this.runtime.botId;
    }
    awaitBotIdentity(timeoutMs) {
        return this.runtime.awaitBotIdentity(timeoutMs);
    }
    get capabilities() {
        return this.runtime.capabilities;
    }
    get superAdmins() {
        return this.runtime.superAdmins;
    }
    get commandPrefixes() {
        return this.runtime.commandPrefixes;
    }
    get primaryCommandPrefix() {
        return this.runtime.primaryCommandPrefix;
    }
    onEvent(eventTypeOrHandler, handler) {
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
    onCommand(name, handler, aliases = []) {
        this.runtime.onCommand(name, bindHandler(this, handler), aliases);
        return this;
    }
    subscribe(...eventTypes) {
        this.runtime.subscribe(...eventTypes);
        return this;
    }
}
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
];
for (const methodName of delegatedRuntimeMethods) {
    Object.defineProperty(RayleaBotPlugin.prototype, methodName, {
        value(...args) {
            const runtime = this.runtime;
            return runtime[methodName](...args);
        },
    });
}
export function createPlugin() {
    return new RayleaBotPlugin();
}
function bindHandler(owner, handler) {
    return handler.bind(owner);
}
async function invokeHandler(plugin, handler, event, requestId) {
    await handler(new PluginEventContext(plugin, event, requestId));
}
function formatErrorMessage(error) {
    if (error instanceof Error) {
        return error.message || error.name;
    }
    return String(error);
}
//# sourceMappingURL=index.js.map