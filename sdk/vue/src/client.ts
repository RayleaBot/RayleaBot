export interface PluginDescriptor {
  id: string
  name: string
  version: string
  state: string
}

export interface PluginPageDescriptor {
  id: string
}

export interface PluginTheme {
  mode: 'light' | 'dark'
  tokens: Record<string, string>
}

export interface HostInitPayload {
  plugin: PluginDescriptor
  page: PluginPageDescriptor
  config: Record<string, unknown>
  secrets_configured: Record<string, boolean>
  theme: PluginTheme
  language: string
}

export interface SettingsChangedPayload {
  config: Record<string, unknown>
}

export interface SecretsStatusPayload {
  configured: Record<string, boolean>
}

export interface PluginUIClientOptions {
  pluginId?: string
  pageId?: string
  fetch?: typeof fetch
}

const pluginUIPath = /^\/plugin-ui\/([^/]+)\//
const csrfHeader = 'X-Raylea-CSRF'

export class PluginUIError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status = 0,
    readonly details?: Record<string, unknown>,
  ) {
    super(message)
    this.name = 'PluginUIError'
  }
}

// PluginUIClient runs inside the same-origin plugin page and calls the
// plugin-scoped management API with the operator session.
export class PluginUIClient {
  readonly pluginId: string
  readonly pageId: string
  private readonly fetcher: typeof fetch
  private csrfToken = ''

  constructor(options: PluginUIClientOptions = {}) {
    this.pluginId = options.pluginId ?? pluginIdFromLocation()
    this.pageId = options.pageId ?? new URLSearchParams(globalThis.location?.search ?? '').get('page') ?? ''
    this.fetcher = options.fetch ?? globalThis.fetch.bind(globalThis)
  }

  async loadContext(): Promise<HostInitPayload> {
    const [detail, settings, secrets] = await Promise.all([
      this.pluginRequest<{ plugin: Partial<PluginDescriptor> & { id: string } }>('GET', ''),
      this.reloadSettings(),
      this.reloadSecretStatus(),
    ])
    return {
      plugin: {
        id: detail.plugin.id,
        name: detail.plugin.name ?? detail.plugin.id,
        version: detail.plugin.version ?? '0.0.0',
        state: detail.plugin.state ?? '',
      },
      page: { id: this.pageId },
      config: settings.config,
      secrets_configured: secrets.configured,
      theme: readHostTheme(),
      language: hostDocument()?.documentElement.lang || globalThis.navigator?.language || 'zh-CN',
    }
  }

  async reloadSettings(): Promise<SettingsChangedPayload> {
    const response = await this.pluginRequest<{ values: Record<string, unknown> }>('GET', '/settings')
    return { config: response.values }
  }

  async saveSettings(values: Record<string, unknown>): Promise<SettingsChangedPayload> {
    const response = await this.pluginRequest<{ values: Record<string, unknown> }>('PUT', '/settings', { values })
    return { config: response.values }
  }

  async reloadSecretStatus(): Promise<SecretsStatusPayload> {
    const response = await this.pluginRequest<SecretsStatusPayload>('GET', '/secrets')
    return { configured: response.configured }
  }

  async setSecrets(values: Record<string, string>): Promise<SecretsStatusPayload> {
    const response = await this.pluginRequest<SecretsStatusPayload>('PUT', '/secrets', { values })
    return { configured: response.configured }
  }

  async deleteSecrets(keys: string[]): Promise<SecretsStatusPayload> {
    const response = await this.pluginRequest<SecretsStatusPayload>('DELETE', '/secrets', { keys })
    return { configured: response.configured }
  }

  async invokeAction(action: string, payload: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
    const response = await this.pluginRequest<{ result: Record<string, unknown> }>('POST', '/management/actions', { action, payload })
    return response.result
  }

  async apiRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
    const upperMethod = method.toUpperCase()
    const stateChanging = !['GET', 'HEAD'].includes(upperMethod)
    if (stateChanging && !this.csrfToken) {
      await this.pluginRequest('GET', '/secrets')
    }
    const headers = new Headers({ Accept: 'application/json' })
    if (body !== undefined) {
      headers.set('Content-Type', 'application/json')
    }
    if (stateChanging && this.csrfToken) {
      headers.set(csrfHeader, this.csrfToken)
    }
    const response = await this.fetcher(path, {
      method: upperMethod,
      headers,
      credentials: 'same-origin',
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    const token = response.headers.get(csrfHeader)
    if (token) {
      this.csrfToken = token
    }
    const payload: unknown = response.status === 204 ? undefined : await response.json().catch(() => undefined)
    if (!response.ok) {
      const envelope = (payload as { error?: { code?: string, message?: string, details?: Record<string, unknown> } } | undefined)?.error
      throw new PluginUIError(
        envelope?.code ?? 'platform.internal_error',
        envelope?.message ?? `${upperMethod} ${path} failed with status ${response.status}`,
        response.status,
        envelope?.details,
      )
    }
    return payload as T
  }

  private pluginRequest<T>(method: string, suffix: string, body?: unknown): Promise<T> {
    return this.apiRequest<T>(method, `/api/plugins/${encodeURIComponent(this.pluginId)}${suffix}`, body)
  }
}

export function readHostTheme(): PluginTheme {
  const root = hostDocument()?.documentElement
  if (!root || typeof getComputedStyle !== 'function') {
    return { mode: 'light', tokens: {} }
  }
  const styles = getComputedStyle(root)
  const token = (...names: string[]) => names.map(name => styles.getPropertyValue(name).trim()).find(Boolean) ?? ''
  return {
    mode: root.dataset.theme === 'dark' ? 'dark' : 'light',
    tokens: {
      'color-bg': token('--bg'),
      'color-surface': token('--surface-strong', '--surface'),
      'color-text': token('--text'),
      'color-muted': token('--muted'),
      'color-primary': token('--brand-fill'),
      'color-border': token('--border'),
    },
  }
}

export function hostDocument(): Document | null {
  if (typeof window === 'undefined') {
    return null
  }
  try {
    return window.parent !== window ? window.parent.document : window.document
  } catch {
    return window.document
  }
}

function pluginIdFromLocation(): string {
  const match = pluginUIPath.exec(globalThis.location?.pathname ?? '')
  if (!match) {
    throw new PluginUIError('plugin.ui_path_invalid', 'The plugin page is not served from a plugin UI path.')
  }
  return decodeURIComponent(match[1])
}
