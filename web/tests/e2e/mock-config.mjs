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

export { redactConfigSecrets }
