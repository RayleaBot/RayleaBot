import { readFrames, requestLocalAction, sendAction, sendError, sendInitAck, sendPong, sendResult, } from './protocol.js';
export { textSegment, imageSegment, atSegment, atAllSegment, faceSegment, replySegment, passthroughSegment, markdownSegment, fileSegment, keyboardSegment, } from './types.js';
export { ActionError } from './protocol.js';
export function createPlugin() {
    const eventHandlers = [];
    const commandHandlers = new Map();
    const activeHandlers = new Set();
    let pluginId = '';
    let botId = '';
    let commandPrefixes = ['/'];
    let subscriptions = null;
    const plugin = {
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
            return await requestOneBotAction(requestId, `provider.${provider}.${action}`, data, options);
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
        async groupAnnouncementCreate(requestId, groupId, content, options = {}) {
            return await requestOneBotAction(requestId, 'group.announcement.create', {
                group_id: groupId,
                content,
            }, options);
        },
        async fileGroupUpload(requestId, groupId, fileName, fileUrl, options = {}) {
            return await requestOneBotAction(requestId, 'file.group.upload', {
                group_id: groupId,
                file_name: fileName,
                file_url: fileUrl,
            }, options);
        },
        async reactionSet(requestId, messageId, emoji, enabled = true, options = {}) {
            return await requestOneBotAction(requestId, 'reaction.set', {
                message_id: messageId,
                emoji,
                enabled,
            }, options);
        },
        async pokeSend(requestId, targetType, targetId, userId, options = {}) {
            return await requestOneBotAction(requestId, 'poke.send', {
                target_type: targetType,
                target_id: targetId,
                user_id: userId,
            }, options);
        },
        async napcatMessageEmojiLikeSet(requestId, messageId, emojiId, enabled = true, options = {}) {
            return await requestOneBotAction(requestId, 'provider.napcat.message_emoji.like.set', {
                message_id: messageId,
                emoji_id: emojiId,
                enabled,
            }, options);
        },
        async luckylilliaFriendGroupsGet(requestId, userId, options = {}) {
            return await requestOneBotAction(requestId, 'provider.luckylillia.friend_groups.get', {
                user_id: userId,
            }, options);
        },
        async run() {
            for await (const frame of readFrames()) {
                const { type, plugin_id, request_id } = frame;
                if (type === 'init') {
                    const initFrame = frame;
                    pluginId = plugin_id;
                    botId = initFrame.bot?.id ?? '';
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
    async function requestOneBotAction(requestId, action, data, options = {}) {
        const { timeoutMs = 30000 } = options;
        return await requestLocalAction(pluginId, requestId, action, data, { timeoutMs });
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