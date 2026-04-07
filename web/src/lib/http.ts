import type { ErrorEnvelope } from '@/types/api'

export class ApiError extends Error {
  code: string
  status: number
  requestId?: string
  details?: Record<string, unknown>
  messageKey?: string

  constructor(
    message: string,
    status: number,
    code = 'platform.unknown',
    requestId?: string,
    details?: Record<string, unknown>,
    messageKey?: string,
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.requestId = requestId
    this.details = details
    this.messageKey = messageKey
  }
}

interface RuntimeConfig {
  getToken: () => string | null
  onUnauthorized: (tokenSnapshot: string | null) => void
}

const runtime: RuntimeConfig = {
  getToken: () => null,
  onUnauthorized: () => undefined,
}

export function configureApiRuntime(config: Partial<RuntimeConfig>) {
  if (config.getToken) {
    runtime.getToken = config.getToken
  }

  if (config.onUnauthorized) {
    runtime.onUnauthorized = config.onUnauthorized
  }
}

export interface ApiRequestOptions extends Omit<RequestInit, 'body'> {
  auth?: boolean
  body?: unknown
  timeoutMs?: number
}

export interface ApiDownloadResult {
  blob: Blob
  filename: string | null
}

const DEFAULT_TIMEOUT_MS = 30_000

function withRuntimeHeaders(
  headers: HeadersInit | undefined,
  auth: boolean,
  hasBody: boolean,
  tokenSnapshot: string | null,
) {
  const requestHeaders = new Headers(headers)

  if (hasBody) {
    requestHeaders.set('Content-Type', 'application/json')
  }

  if (auth && tokenSnapshot) {
    requestHeaders.set('Authorization', `Bearer ${tokenSnapshot}`)
  }

  return requestHeaders
}

function parseDownloadFilename(contentDisposition: string | null) {
  if (!contentDisposition) {
    return null
  }

  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1])
  }

  const quotedMatch = contentDisposition.match(/filename=\"([^\"]+)\"/i)
  if (quotedMatch?.[1]) {
    return quotedMatch[1]
  }

  return null
}

export async function apiRequest<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const { auth = true, headers, body, timeoutMs = DEFAULT_TIMEOUT_MS, signal: callerSignal, ...rest } = options
  const tokenSnapshot = auth ? runtime.getToken() : null
  const requestHeaders = withRuntimeHeaders(headers, auth, body !== undefined, tokenSnapshot)

  const controller = new AbortController()
  const timeoutId = timeoutMs > 0 ? setTimeout(() => controller.abort(), timeoutMs) : undefined
  callerSignal?.addEventListener('abort', () => controller.abort(), { once: true })

  let response: Response
  try {
    response = await fetch(path, {
      ...rest,
      signal: controller.signal,
      headers: requestHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId)
  }

  if (response.status === 204) {
    return undefined as T
  }

  const contentType = response.headers.get('content-type') ?? ''
  const isJson = contentType.includes('application/json')
  const payload = isJson ? await response.json() : await response.text()

  if (!response.ok) {
    const errorEnvelope = typeof payload === 'object' && payload !== null && 'error' in payload
      ? (payload as ErrorEnvelope)
      : undefined

    if (response.status === 401 && auth) {
      runtime.onUnauthorized(tokenSnapshot)
    }

    throw new ApiError(
      errorEnvelope?.error.message ?? response.statusText,
      response.status,
      errorEnvelope?.error.code,
      errorEnvelope?.error.request_id,
      errorEnvelope?.error.details,
      errorEnvelope?.error.message_key,
    )
  }

  return payload as T
}

export async function apiDownload(path: string, options: ApiRequestOptions = {}): Promise<ApiDownloadResult> {
  const { auth = true, headers, body, timeoutMs = DEFAULT_TIMEOUT_MS, signal: callerSignal, ...rest } = options
  const tokenSnapshot = auth ? runtime.getToken() : null
  const requestHeaders = withRuntimeHeaders(headers, auth, body !== undefined, tokenSnapshot)

  const controller = new AbortController()
  const timeoutId = timeoutMs > 0 ? setTimeout(() => controller.abort(), timeoutMs) : undefined
  callerSignal?.addEventListener('abort', () => controller.abort(), { once: true })

  let response: Response
  try {
    response = await fetch(path, {
      ...rest,
      signal: controller.signal,
      headers: requestHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId)
  }

  if (!response.ok) {
    const contentType = response.headers.get('content-type') ?? ''
    const isJson = contentType.includes('application/json')
    const payload = isJson ? await response.json() : await response.text()
    const errorEnvelope = typeof payload === 'object' && payload !== null && 'error' in payload
      ? (payload as ErrorEnvelope)
      : undefined

    if (response.status === 401 && auth) {
      runtime.onUnauthorized(tokenSnapshot)
    }

    throw new ApiError(
      errorEnvelope?.error.message ?? response.statusText,
      response.status,
      errorEnvelope?.error.code,
      errorEnvelope?.error.request_id,
      errorEnvelope?.error.details,
      errorEnvelope?.error.message_key,
    )
  }

  return {
    blob: await response.blob(),
    filename: parseDownloadFilename(response.headers.get('content-disposition')),
  }
}
