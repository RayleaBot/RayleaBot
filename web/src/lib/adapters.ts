import type { AdapterProtocol, ConfigDocument } from '@/types/api'

// One configured adapter instance as it appears in the config document. The
// settings block is named by the protocol, so only the block matching `type`
// is present on any one instance.
export type AdapterInstanceDocument = ConfigDocument['adapters'][number]
export type OneBotSettings = NonNullable<AdapterInstanceDocument['onebot11']>
export type QQOfficialSettings = NonNullable<AdapterInstanceDocument['qqofficial']>
export type OneBotTransport = keyof OneBotSettings

export function readAdapterInstances(document: ConfigDocument | null): AdapterInstanceDocument[] {
  return document?.adapters ?? []
}

export function findAdapterInstance(document: ConfigDocument | null, id: string) {
  return readAdapterInstances(document).find((instance) => instance.id === id) ?? null
}

const DEFAULT_ONEBOT_WS_TRANSPORT = {
  enabled: false,
  url: '',
  access_token: '',
  access_token_query_compat: false,
}

function defaultAdapterSettings(protocol: 'onebot11'): OneBotSettings
function defaultAdapterSettings(protocol: 'qqofficial'): QQOfficialSettings
function defaultAdapterSettings(protocol: AdapterProtocol): OneBotSettings | QQOfficialSettings {
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
    ...(protocol === 'onebot11'
      ? { onebot11: defaultAdapterSettings(protocol) }
      : { qqofficial: defaultAdapterSettings(protocol) }),
  }
}
