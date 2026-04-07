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
} from './types.js';
export { ActionError } from './protocol.js';

type EventHandler = (event: EventBody, requestId: string) => void | Promise<void>;

interface ActionOptions {
  timeoutMs?: number;
}

export interface RayleaBotPlugin {
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

  run(): Promise<void>;
}

export function createPlugin(): RayleaBotPlugin {
  const eventHandlers: Array<{ type: string | null; handler: EventHandler }> = [];
  const commandHandlers = new Map<string, EventHandler>();
  let pluginId = '';
  let botId = '';
  let subscriptions: string[] | null = null;

  const plugin: RayleaBotPlugin = {
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

    async run(): Promise<void> {
      for await (const frame of readFrames()) {
        const { type, plugin_id, request_id } = frame;

        if (type === 'init') {
          const initFrame = frame as InitFrame;
          pluginId = plugin_id;
          botId = initFrame.bot?.id ?? '';
          sendInitAck(pluginId, request_id, subscriptions);
        } else if (type === 'event') {
          const eventFrame = frame as EventFrame;
          await handleEvent(eventFrame, plugin_id, request_id);
        } else if (type === 'ping') {
          sendPong(pluginId, request_id);
        } else if (type === 'shutdown') {
          break;
        }
      }
    },
  };

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

  return plugin;
}
