import {
  readFrames,
  requestLocalAction,
  sendAction,
  sendInitAck,
  sendPong,
  sendResult,
} from './protocol.js';

export function createPlugin() {
  const eventHandlers = [];
  const commandHandlers = new Map();
  let pluginId = '';
  let botId = '';
  let subscriptions = null;

  const plugin = {
    onEvent(eventType, handler) {
      if (typeof eventType === 'function') {
        eventHandlers.push({ type: null, handler: eventType });
      } else {
        eventHandlers.push({ type: eventType, handler });
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

    sendMessage(requestId, targetType, targetId, text) {
      sendAction(pluginId, requestId, 'message.send', { target_type: targetType, target_id: targetId, text });
    },

    sendReply(requestId, replyToMessageId, text) {
      sendAction(pluginId, requestId, 'message.reply', { reply_to_message_id: replyToMessageId, text });
    },

    sendMessageSegments(requestId, targetType, targetId, segments) {
      sendAction(pluginId, requestId, 'message.send', {
        target_type: targetType,
        target_id: targetId,
        message: { segments },
      });
    },

    replyToEvent(requestId, replyToEventId, segments, options = {}) {
      const data = {
        reply_to_event_id: replyToEventId,
        message: { segments },
      };
      if (options.fallbackToSendIfMissing) {
        data.fallback_to_send_if_missing = true;
      }
      sendAction(pluginId, requestId, 'message.reply', data);
    },

    sendImage(requestId, targetType, targetId, file) {
      sendAction(pluginId, requestId, 'message.send_image', { target_type: targetType, target_id: targetId, file });
    },

    async loggerWrite(requestId, level, message, fields = undefined, options = {}) {
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

    async run() {
      for await (const frame of readFrames()) {
        const { type, plugin_id, request_id } = frame;

        if (type === 'init') {
          pluginId = plugin_id;
          botId = frame.bot?.id ?? '';
          sendInitAck(pluginId, request_id, subscriptions);
        } else if (type === 'event') {
          await handleEvent(frame, plugin_id, request_id);
        } else if (type === 'ping') {
          sendPong(pluginId, request_id);
        } else if (type === 'shutdown') {
          break;
        }
      }
    },
  };

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

  return plugin;
}
