import { readFrame, writeFrame, sendInitAck, sendPong, sendResult, sendAction } from './protocol.js';

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

    sendImage(requestId, targetType, targetId, file) {
      sendAction(pluginId, requestId, 'message.send_image', { target_type: targetType, target_id: targetId, file });
    },

    async run() {
      for await (const frame of readFrame()) {
        const { type, plugin_id, request_id } = frame;

        if (type === 'init') {
          pluginId = plugin_id;
          botId = frame.bot?.id ?? '';
          sendInitAck(pluginId, request_id, subscriptions);
        } else if (type === 'event') {
          handleEvent(frame, plugin_id, request_id);
        } else if (type === 'ping') {
          sendPong(pluginId, request_id);
        } else if (type === 'shutdown') {
          break;
        }
      }
    },
  };

  function handleEvent(frame, pid, requestId) {
    const event = frame.event ?? {};
    const command = event.payload?.command;

    if (command && commandHandlers.has(command)) {
      commandHandlers.get(command)(event, requestId);
      return;
    }

    for (const { type, handler } of eventHandlers) {
      if (type === null || type === event.event_type) {
        handler(event, requestId);
        return;
      }
    }

    sendResult(pid, requestId, { handled: false });
  }

  return plugin;
}
