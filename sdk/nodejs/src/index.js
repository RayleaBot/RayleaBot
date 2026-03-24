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

    async storageFileRead(requestId, path, options = {}) {
      const root = options.root ?? 'plugin_data';
      const { timeoutMs = 30000 } = options;
      return await requestLocalAction(pluginId, requestId, 'storage.file', { operation: 'read', root, path }, { timeoutMs });
    },

    async storageFileWrite(requestId, path, { root = 'plugin_data', contentText, contentBase64, timeoutMs = 30000 } = {}) {
      if ((contentText === undefined) === (contentBase64 === undefined)) {
        throw new Error('storageFileWrite requires exactly one of contentText or contentBase64');
      }
      const data = { operation: 'write', root, path };
      if (contentText !== undefined) {
        data.content_text = contentText;
      } else {
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
      const data = { method, url };
      const { headers, timeoutMs = 30000, timeoutSeconds, bodyText, bodyBase64 } = options;
      if (bodyText !== undefined && bodyBase64 !== undefined) {
        throw new Error('httpRequest requires at most one of bodyText or bodyBase64');
      }
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

    async exposeWebhook(requestId, route, options = {}) {
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
      const {
        theme,
        output,
        fallbackText,
        timeoutMs = 30000,
      } = options;
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
