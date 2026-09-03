export function readInternalRedirectTarget(value: unknown): string | null {
  const candidate = Array.isArray(value) ? value[0] : value
  if (typeof candidate !== 'string' || !candidate.trim()) {
    return null
  }

  if (!candidate.startsWith('/') || candidate.startsWith('//') || /\\/.test(candidate)) {
    return null
  }

  return candidate
}
