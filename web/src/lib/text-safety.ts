const unsafeDisplayControlPattern = /[\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F-\u009F\u061C\u200E\u200F\u2028\u2029\u202A-\u202E\u2066-\u206F\uFEFF]/g

export function escapeUnsafeDisplayText(value: string) {
  return value.replace(unsafeDisplayControlPattern, (char) => (
    `\\u${char.codePointAt(0)?.toString(16).padStart(4, '0') ?? 'fffd'}`
  ))
}

export function toSafeDisplayText(value: unknown) {
  if (value === null || value === undefined) {
    return ''
  }

  return escapeUnsafeDisplayText(String(value))
}

export function safeJsonStringify(value: unknown) {
  try {
    return escapeUnsafeDisplayText(JSON.stringify(value ?? {}, null, 2))
  } catch {
    return '{}'
  }
}
