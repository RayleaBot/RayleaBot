export interface PluginDescriptor {
  id: string
  name: string
  version: string
  state: string
}

export type PluginPageScope = 'global' | 'bot_account'

export interface PluginPageDescriptor {
  id: string
  scope: PluginPageScope
}

export type BotAccountProtocol = 'onebot11' | 'qqofficial'

// BotAccountRef names a bot account the way delivery addresses and access
// rules do. It is the only account data the page URL and actions carry.
export interface BotAccountRef {
  source_protocol: BotAccountProtocol
  bot_id: string
}

export type BotAccountState = 'online' | 'offline' | 'no_connection'

// BotAccount is the host's directory entry. Its profile and connection can
// change while the page is open, so pages display it but never deliver by it.
export interface BotAccount extends BotAccountRef {
  nickname?: string
  avatar_url?: string
  state: BotAccountState
  adapter_id?: string
  adapter_display_name?: string
  last_adapter_id?: string
  last_adapter_display_name?: string
  last_seen_at: string
}

export interface PluginTheme {
  mode: 'light' | 'dark'
  tokens: Record<string, string>
}

export interface HostInitPayload {
  plugin: PluginDescriptor
  page: PluginPageDescriptor
  // The selected account on a bot_account page; null on global pages.
  bot: BotAccount | null
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
  // The selected account; read from the page URL when omitted.
  bot?: BotAccountRef | null
  fetch?: typeof fetch
}

interface PluginDetailPages {
  plugin: Partial<PluginDescriptor> & {
    id: string
    management_ui?: { pages?: Array<{ id: string, scope?: PluginPageScope }> }
  }
}

const pluginUIPath = /^\/plugin-ui\/([^/]+)\//
const botAccountProtocols: readonly string[] = ['onebot11', 'qqofficial']
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
  // The account the page was opened for. It never changes: the host reloads
  // the page to switch accounts.
  readonly botAccount: BotAccountRef | null
  private readonly fetcher: typeof fetch
  private csrfToken = ''
  private pageScope: Promise<PluginPageScope> | null = null

  constructor(options: PluginUIClientOptions = {}) {
    const search = new URLSearchParams(globalThis.location?.search ?? '')
    this.pluginId = options.pluginId ?? pluginIdFromLocation()
    this.pageId = options.pageId ?? search.get('page') ?? ''
    this.botAccount = options.bot !== undefined ? options.bot : parseBotAccountKey(search.getAll('bot'))
    this.fetcher = options.fetch ?? globalThis.fetch.bind(globalThis)
  }

  async loadContext(): Promise<HostInitPayload> {
    const [detail, settings, secrets] = await Promise.all([
      this.pluginRequest<PluginDetailPages>('GET', ''),
      this.reloadSettings(),
      this.reloadSecretStatus(),
    ])
    const scope = declaredPageScope(detail, this.pageId)
    this.pageScope = Promise.resolve(scope)
    const bot = scope === 'bot_account' ? await this.reloadBotAccount() : null
    return {
      plugin: {
        id: detail.plugin.id,
        name: detail.plugin.name ?? detail.plugin.id,
        version: detail.plugin.version ?? '0.0.0',
        state: detail.plugin.state ?? '',
      },
      page: { id: this.pageId, scope },
      bot,
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

  // reloadBotAccount reads the selected account from the host directory, for
  // example before an online lookup that needs its current connection.
  async reloadBotAccount(): Promise<BotAccount> {
    const account = this.requireBotAccount()
    return this.apiRequest<BotAccount>(
      'GET',
      `/api/bot-accounts/${encodeURIComponent(account.source_protocol)}/${encodeURIComponent(account.bot_id)}`,
    )
  }

  // invokeAction sends the page and the selected account with every action of
  // a bot_account page; global pages send the action unchanged.
  async invokeAction(action: string, payload: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
    const body: Record<string, unknown> = { action, payload }
    if (await this.resolvePageScope() === 'bot_account') {
      body.page_id = this.pageId
      body.bot = this.requireBotAccount()
    }
    const response = await this.pluginRequest<{ result: Record<string, unknown> }>('POST', '/management/actions', body)
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

  private resolvePageScope(): Promise<PluginPageScope> {
    this.pageScope ??= this.pluginRequest<PluginDetailPages>('GET', '').then(detail => declaredPageScope(detail, this.pageId))
    return this.pageScope
  }

  private requireBotAccount(): BotAccountRef {
    if (!this.botAccount) {
      throw new PluginUIError('plugin.ui_bot_account_missing', 'This page belongs to a bot account, but no valid account was selected.')
    }
    return this.botAccount
  }
}

// parseBotAccountKey reads the host's "<source_protocol>:<bot_id>" value. The
// account part is used verbatim, so only the first colon separates the two.
export function parseBotAccountKey(values: string | readonly string[] | null): BotAccountRef | null {
  const list = typeof values === 'string' ? [values] : values ?? []
  if (list.length !== 1) {
    return null
  }
  const value = list[0]
  const separator = value.indexOf(':')
  const protocol = value.slice(0, separator)
  const botID = value.slice(separator + 1)
  if (separator <= 0 || !botID || !botAccountProtocols.includes(protocol)) {
    return null
  }
  return { source_protocol: protocol as BotAccountProtocol, bot_id: botID }
}

export function formatBotAccountKey(account: BotAccountRef): string {
  return `${account.source_protocol}:${account.bot_id}`
}

function declaredPageScope(detail: PluginDetailPages, pageId: string): PluginPageScope {
  return detail.plugin.management_ui?.pages?.find(page => page.id === pageId)?.scope ?? 'global'
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
