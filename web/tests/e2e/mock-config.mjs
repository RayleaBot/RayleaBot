const redactedConfigValue = '********'

function secretConfigPaths(config) {
  return (config.adapters ?? []).flatMap((entry) => entry.type === 'onebot11'
    ? ['forward_ws', 'http_api', 'reverse_ws', 'webhook'].map((transport) => ['adapters', entry.id, 'onebot11', transport, 'access_token'])
    : [['adapters', entry.id, 'qqofficial', 'app_secret']])
}



function redactConfigSecrets(config) {
  const snapshot = structuredClone(config)
  const redactedFields = []
  for (const secretPath of secretConfigPaths(config)) {
    const value = getPath(snapshot, secretPath)
    if (typeof value !== 'string' || value.trim() === '') {
      continue
    }
    setPath(snapshot, secretPath, redactedConfigValue)
    redactedFields.push(secretPath.join('.'))
  }
  return {
    config: snapshot,
    redacted_fields: redactedFields.sort(),
  }
}

function restoreRedactedConfigSecrets(payload, currentConfig) {
  const nextConfig = structuredClone(payload)
  for (const secretPath of secretConfigPaths(nextConfig)) {
    const submitted = getPath(nextConfig, secretPath)
    if (submitted !== undefined && String(submitted).trim() !== redactedConfigValue) {
      continue
    }
    setPath(nextConfig, secretPath, String(getPath(currentConfig, secretPath) ?? ''))
  }
  return nextConfig
}

function getPath(value, segments) {
  let current = value
  for (const segment of segments) {
    if (Array.isArray(current)) {
      current = current.find((entry) => entry.id === segment)
      continue
    }
    if (!current || typeof current !== 'object' || !(segment in current)) {
      return undefined
    }
    current = current[segment]
  }
  return current
}

function setPath(value, segments, nextValue) {
  let current = value
  for (const segment of segments.slice(0, -1)) {
    if (Array.isArray(current)) {
      current = current.find((entry) => entry.id === segment)
      if (!current) return
      continue
    }
    if (!current[segment] || typeof current[segment] !== 'object') {
      current[segment] = {}
    }
    current = current[segment]
  }
  current[segments.at(-1)] = nextValue
}

function normalizeTransport(entry = {}) {
  return {
    enabled: Boolean(entry.enabled),
    url: String(entry.url ?? ''),
    access_token: String(entry.access_token ?? ''),
  }
}

function pickOneBotHotState(config) {
  const onebot = config.onebot ?? {}
  const adapter = config.adapter ?? {}

  return {
    adapter: {
      connect_timeout_seconds: adapter.connect_timeout_seconds ?? 0,
      reconnect_initial_seconds: adapter.reconnect_initial_seconds ?? 0,
      reconnect_multiplier: adapter.reconnect_multiplier ?? 0,
      reconnect_max_seconds: adapter.reconnect_max_seconds ?? 0,
      reconnect_jitter_ratio: adapter.reconnect_jitter_ratio ?? 0,
    },
    onebot: {
      reverse_ws: normalizeTransport(onebot.reverse_ws),
      forward_ws: normalizeTransport(onebot.forward_ws),
      http_api: normalizeTransport(onebot.http_api),
      webhook: normalizeTransport(onebot.webhook),
    },
  }
}

function computeRestartRequiredForConfig(prevConfig, nextConfig) {
  return computeConfigApplyEffects(prevConfig, nextConfig).restart_required_fields.length > 0
}

const configRestartRequiredFields = new Set([
  'adapters',
  'admin.max_sessions',
  'admin.session_ttl_days',
  'admin.sliding_renewal',
  'database.engine',
  'database.path',
  'render.browser_args',
  'render.browser_path',
  'render.worker_count',
  'scheduler.timezone',
  'server.host',
  'server.port',
  'third_party_accounts.douyin_login.browser_mode',
  'third_party_accounts.douyin_login.remote_debugging_url',
  'web.exposure_mode',
  'web.setup_local_only',
])

function computeConfigApplyEffects(prevConfig, nextConfig) {
  const changedPaths = []
  collectChangedConfigPaths('', prevConfig ?? {}, nextConfig ?? {}, changedPaths)
  changedPaths.sort()

  const effects = {
    applied_now: [],
    reloaded_now: [],
    restart_required_fields: [],
  }

  for (const path of [...new Set(changedPaths)]) {
    if (path.startsWith('adapters.') || path.startsWith('adapter.')) {
      effects.reloaded_now.push(path)
    } else if (configRestartRequiredFields.has(path) || path.startsWith('database.') || path.startsWith('server.') || path.startsWith('web.') || path.startsWith('runtime.')) {
      effects.restart_required_fields.push(path)
    } else {
      effects.applied_now.push(path)
    }
  }

  return effects
}

function collectChangedConfigPaths(prefix, prevValue, nextValue, changedPaths) {
  if (prefix === 'adapters' && Array.isArray(prevValue) && Array.isArray(nextValue)) {
    const identities = (entries) => entries.map(({ id, type }) => ({ id, type }))
    if (JSON.stringify(identities(prevValue)) !== JSON.stringify(identities(nextValue))) {
      changedPaths.push(prefix)
    } else {
      nextValue.forEach((entry, index) => collectChangedConfigPaths(`adapters.${entry.id}`, prevValue[index], entry, changedPaths))
    }
    return
  }
  if (isPlainObject(prevValue) && isPlainObject(nextValue)) {
    const keys = [...new Set([...Object.keys(prevValue), ...Object.keys(nextValue)])].sort()
    for (const key of keys) {
      collectChangedConfigPaths(prefix ? `${prefix}.${key}` : key, prevValue[key], nextValue[key], changedPaths)
    }
    return
  }

  if (prefix && JSON.stringify(prevValue) !== JSON.stringify(nextValue)) {
    changedPaths.push(prefix)
  }
}

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

export { redactConfigSecrets, restoreRedactedConfigSecrets, computeRestartRequiredForConfig, computeConfigApplyEffects }
