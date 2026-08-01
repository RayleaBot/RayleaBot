import type {
  BridgeErrorPayload,
  BridgeMessage,
  BridgeType,
  HostInitPayload,
  SecretsStatusPayload,
  SettingsChangedPayload,
} from './contract.generated'

export const PLUGIN_UI_BRIDGE_VERSION = '2' as const
const minimumHeight = 320
const maximumHeight = 1600

type MessageHandler = (message: BridgeMessage) => void

export class PluginUIBridgeError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly details?: Record<string, unknown>,
  ) {
    super(message)
    this.name = 'PluginUIBridgeError'
  }
}

export class PluginUIBridgeClient {
  private port: MessagePort | null = null
  private connected: Promise<HostInitPayload> | null = null
  private readonly listeners = new Map<BridgeType, Set<MessageHandler>>()
  private readonly pending = new Map<string, {
    resolve: (message: BridgeMessage) => void
    reject: (error: Error) => void
    timer: number
  }>()
  private sequence = 0
  private resizeObserver: ResizeObserver | null = null

  connect(): Promise<HostInitPayload> {
    if (this.connected) {
      return this.connected
    }
    this.connected = this.performConnect()
    return this.connected
  }

  private performConnect(): Promise<HostInitPayload> {
    const nonce = readBridgeNonce()
    const parentOrigin = resolveParentOrigin()
    return new Promise((resolve, reject) => {
      const timeout = globalThis.setTimeout(() => {
        window.removeEventListener('message', onWindowMessage)
        reject(new PluginUIBridgeError('plugin.bridge_timeout', 'Timed out waiting for the management host.'))
      }, 10_000)
      const onWindowMessage = (event: MessageEvent) => {
        const message = parseMessage(event.data)
        if (
          event.source !== window.parent
          || event.origin !== parentOrigin
          || message?.type !== 'host.connect'
          || message.source !== 'management_host'
          || (event.data as { nonce?: string }).nonce !== nonce
          || event.ports.length !== 1
        ) {
          return
        }
        globalThis.clearTimeout(timeout)
        window.removeEventListener('message', onWindowMessage)
        this.port = event.ports[0]
        this.port.addEventListener('message', this.handlePortMessage)
        this.port.start()
        const stop = this.on('host.init', (initMessage) => {
          stop()
          resolve(initMessage.payload as HostInitPayload)
        })
      }
      window.addEventListener('message', onWindowMessage)
      window.parent.postMessage({
        version: PLUGIN_UI_BRIDGE_VERSION,
        source: 'plugin_management_ui',
        type: 'page.ready',
        nonce,
      }, parentOrigin)
    })
  }

  private readonly handlePortMessage = (event: MessageEvent) => {
    const message = parseMessage(event.data)
    if (!message || message.source !== 'management_host' || message.type === 'host.connect') {
      return
    }
    if (message.request_id) {
      const pending = this.pending.get(message.request_id)
      if (pending) {
        this.pending.delete(message.request_id)
        globalThis.clearTimeout(pending.timer)
        if (message.type === 'error') {
          const payload = message.payload as BridgeErrorPayload
          pending.reject(new PluginUIBridgeError(payload.code, payload.message, payload.details))
        } else {
          pending.resolve(message)
        }
      }
    }
    for (const listener of this.listeners.get(message.type) ?? []) {
      listener(message)
    }
  }

  on(type: BridgeType, handler: MessageHandler): () => void {
    const handlers = this.listeners.get(type) ?? new Set<MessageHandler>()
    handlers.add(handler)
    this.listeners.set(type, handlers)
    return () => handlers.delete(handler)
  }

  send(type: BridgeType, payload?: unknown, requestId?: string): void {
    if (!this.port) {
      throw new PluginUIBridgeError('plugin.bridge_unavailable', 'The management host channel is not connected.')
    }
    const message: BridgeMessage = {
      version: PLUGIN_UI_BRIDGE_VERSION,
      source: 'plugin_management_ui',
      type,
    }
    if (payload !== undefined) {
      message.payload = payload
    }
    if (requestId) {
      message.request_id = requestId
    }
    this.port.postMessage(message)
  }

  request<T>(type: BridgeType, payload?: unknown, timeoutMs = 15_000): Promise<T> {
    const requestId = `plugin-ui-${Date.now()}-${++this.sequence}`
    return new Promise<T>((resolve, reject) => {
      const timer = globalThis.setTimeout(() => {
        this.pending.delete(requestId)
        reject(new PluginUIBridgeError('plugin.bridge_timeout', `${type} timed out.`))
      }, timeoutMs)
      this.pending.set(requestId, {
        resolve: (message) => resolve(message.payload as T),
        reject,
        timer,
      })
      try {
        this.send(type, payload, requestId)
      } catch (error) {
        globalThis.clearTimeout(timer)
        this.pending.delete(requestId)
        reject(error)
      }
    })
  }

  reloadSettings(): Promise<SettingsChangedPayload> {
    return this.request('settings.reload')
  }

  saveSettings(values: Record<string, unknown>): Promise<SettingsChangedPayload> {
    return this.request('settings.save', { config: values })
  }

  reloadSecretStatus(): Promise<SecretsStatusPayload> {
    return this.request('secrets.status.reload')
  }

  setSecrets(values: Record<string, string>): Promise<SecretsStatusPayload> {
    return this.request('secrets.set', { values })
  }

  deleteSecrets(keys: string[]): Promise<SecretsStatusPayload> {
    return this.request('secrets.delete', { keys })
  }

  invokeAction(action: string, payload: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
    return this.request<{ action: string; result: Record<string, unknown> }>('plugin.action.invoke', { action, payload })
      .then((response) => response.result)
  }

  reportHeight(height: number): void {
    const bounded = Math.min(maximumHeight, Math.max(minimumHeight, Math.ceil(height)))
    this.send('ui.resize', { height: bounded })
  }

  observeDocumentHeight(): () => void {
    this.resizeObserver?.disconnect()
    const report = () => this.reportHeight(document.documentElement.scrollHeight)
    this.resizeObserver = new ResizeObserver(report)
    this.resizeObserver.observe(document.documentElement)
    report()
    return () => {
      this.resizeObserver?.disconnect()
      this.resizeObserver = null
    }
  }

  close(): void {
    this.resizeObserver?.disconnect()
    this.port?.close()
    this.port = null
    for (const pending of this.pending.values()) {
      globalThis.clearTimeout(pending.timer)
      pending.reject(new PluginUIBridgeError('plugin.bridge_closed', 'The management host channel closed.'))
    }
    this.pending.clear()
  }
}

function parseMessage(value: unknown): BridgeMessage | null {
  if (!value || typeof value !== 'object') {
    return null
  }
  const message = value as Partial<BridgeMessage>
  if (message.version !== PLUGIN_UI_BRIDGE_VERSION || typeof message.type !== 'string' || typeof message.source !== 'string') {
    return null
  }
  return message as BridgeMessage
}

function readBridgeNonce(): string {
  const nonce = new URL(window.location.href).searchParams.get('bridge_nonce')?.trim() ?? ''
  if (!/^[A-Za-z0-9_-]{32,128}$/.test(nonce)) {
    throw new PluginUIBridgeError('plugin.bridge_nonce_invalid', 'The management host did not provide a valid bridge nonce.')
  }
  return nonce
}

function resolveParentOrigin(): string {
  const ancestorOrigin = window.location.ancestorOrigins?.[0]
  if (ancestorOrigin) {
    return ancestorOrigin
  }
  if (document.referrer) {
    return new URL(document.referrer).origin
  }
  throw new PluginUIBridgeError('plugin.bridge_origin_missing', 'The management host origin is unavailable.')
}
