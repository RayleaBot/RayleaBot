import { readFrames, requestLocalAction, sendAction, sendError, sendInitAck, sendPong, sendResult, } from './protocol.js';
export { textSegment, imageSegment, atSegment, atAllSegment, faceSegment, replySegment, passthroughSegment, recordSegment, videoSegment, markdownSegment, fileSegment, flashFileSegment, jsonSegment, xmlSegment, musicSegment, contactSegment, forwardSegment, nodeSegment, pokeSegment, diceSegment, rpsSegment, mfaceSegment, keyboardSegment, shakeSegment, } from './types.js';
export { ActionError } from './protocol.js';
export function createPlugin() {
    const eventHandlers = [];
    const commandHandlers = new Map();
    const activeHandlers = new Set();
    let pluginId = '';
    let botId = '';
    let capabilities = [];
    let commandPrefixes = ['/'];
    let subscriptions = null;
    const plugin = {
        get botId() {
            return botId;
        },
        get capabilities() {
            return [...capabilities];
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
            const { payload, timeoutMs = 30000 } = options;
            const data = {
                task_id: taskId,
                cron,
                event_type: 'scheduler.trigger',
            };
            if (payload !== undefined) {
                data.payload = payload;
            }
            return await requestLocalAction(pluginId, requestId, 'scheduler.create', data, { timeoutMs });
        },
        async exposeWebhook(requestId, route, options = { secretRef: '' }) {
            const { methods = ['POST'], authStrategy = 'fixed_token', header = 'X-Webhook-Token', secretRef, signaturePrefix, sourceIps, timeoutMs = 30000, } = options;
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
                    botId = initFrame.bot?.id ?? '';
                    capabilities = Array.isArray(initFrame.capabilities)
                        ? initFrame.capabilities.filter((value) => typeof value === 'string' && value.length > 0)
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
            await commandHandlers.get(command)(event, requestId);
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
    function updateBotIdentity(event) {
        if (event.event_type !== 'bot.identity.changed') {
            return;
        }
        const targetId = event.target?.type === 'bot' ? event.target.id : undefined;
        const selfId = event.payload?.onebot?.self_id;
        botId = targetId || selfId || botId;
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
function formatErrorMessage(error) {
    if (error instanceof Error) {
        return error.message || error.name;
    }
    return String(error);
}
//# sourceMappingURL=index.js.map