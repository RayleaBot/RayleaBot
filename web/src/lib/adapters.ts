import type { AdapterProtocol, ConfigDocument } from '@/types/api'

// One configured adapter instance as it appears in the config document. The
// settings block is named by the protocol, so only the block matching `type`
// is present on any one instance.
export interface AdapterInstanceDocument {
  id: string
  type: AdapterProtocol
  enabled: boolean
  onebot11?: Record<string, unknown>
  qqofficial?: Record<string, unknown>
  [key: string]: unknown
}

export function readAdapterInstances(document: ConfigDocument | null): AdapterInstanceDocument[] {
  const adapters = (document as Record<string, unknown> | null)?.adapters
  return Array.isArray(adapters) ? (adapters as AdapterInstanceDocument[]) : []
}

export function findAdapterInstance(document: ConfigDocument | null, id: string) {
  return readAdapterInstances(document).find((instance) => instance.id === id) ?? null
}

// adapterSettingsPath spells where one instance's settings live in the config
// document. Instances are addressed by identifier rather than position, so
// reordering the list does not move a field.
export function adapterSettingsPath(id: string, protocol: AdapterProtocol, field: string) {
  return `adapters.${id}.${protocol}.${field}`
}

const DEFAULT_ONEBOT_WS_TRANSPORT = {
  enabled: false,
  url: '',
  access_token: '',
  access_token_query_compat: false,
}

// defaultAdapterSettings builds a settings block that satisfies the schema's
// required fields, so a newly added instance saves before it is configured.
export function defaultAdapterSettings(protocol: AdapterProtocol): Record<string, unknown> {
  if (protocol === 'onebot11') {
    return {
      reverse_ws: { ...DEFAULT_ONEBOT_WS_TRANSPORT },
      forward_ws: { ...DEFAULT_ONEBOT_WS_TRANSPORT },
      http_api: { enabled: false, url: '', access_token: '' },
      webhook: { ...DEFAULT_ONEBOT_WS_TRANSPORT },
    }
  }
  return { app_id: '', app_secret: '', intents: [], sandbox: false }
}

// nextAdapterInstanceId names a new instance after its protocol, and numbers it
// only when that name is taken. The identifier is part of the reverse-WebSocket
// URL and of the secret store keys, so it stays short and stable.
export function nextAdapterInstanceId(document: ConfigDocument | null, protocol: AdapterProtocol) {
  const base = protocol === 'qqofficial' ? 'qq-official' : protocol
  const taken = new Set(readAdapterInstances(document).map((instance) => instance.id))
  if (!taken.has(base)) {
    return base
  }
  for (let index = 2; ; index += 1) {
    const candidate = `${base}-${index}`
    if (!taken.has(candidate)) {
      return candidate
    }
  }
}

export function buildAdapterInstance(id: string, protocol: AdapterProtocol): AdapterInstanceDocument {
  return {
    id,
    type: protocol,
    enabled: false,
    [protocol]: defaultAdapterSettings(protocol),
  } as AdapterInstanceDocument
}

export function adapterEditorRoute(protocol: AdapterProtocol, id: string) {
  return `/protocols/${protocol}/${encodeURIComponent(id)}`
}
