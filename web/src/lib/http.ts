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
  onNetworkUnavailable: (path: string, error: Error) => void
  onReachable: (path: string, status: number) => void
  onUnauthorized: (tokenSnapshot: string | null) => void
}

const runtime: RuntimeConfig = {
  getToken: () => null,
  onNetworkUnavailable: () => undefined,
  onReachable: () => undefined,
  onUnauthorized: () => undefined,
}

export function configureApiRuntime(config: Partial<RuntimeConfig>) {
  if (config.getToken) {
    runtime.getToken = config.getToken
  }

  if (config.onNetworkUnavailable) {
    runtime.onNetworkUnavailable = config.onNetworkUnavailable
  }

  if (config.onReachable) {
    runtime.onReachable = config.onReachable
  }

  if (config.onUnauthorized) {
    runtime.onUnauthorized = config.onUnauthorized
  }
}

export interface ApiRequestOptions extends Omit<RequestInit, 'body'> {
  auth?: boolean
  body?: unknown
  timeoutMs?: number
  acceptStatuses?: number[]
}

export interface ApiDownloadResult {
  blob: Blob
  filename: string | null
}

const DEFAULT_TIMEOUT_MS = 30_000
const backendUnavailableHeader = 'x-rayleabot-backend-unavailable'

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
    return decodeFilenamePart(utf8Match[1])
  }

  const quotedMatch = contentDisposition.match(/filename=\"([^\"]+)\"/i)
  if (quotedMatch?.[1]) {
    return decodeFilenamePart(quotedMatch[1])
  }

  const plainMatch = contentDisposition.match(/filename=([^;\s]+)/i)
  if (plainMatch?.[1]) {
    return decodeFilenamePart(plainMatch[1])
  }

  return null
}

function decodeFilenamePart(value: string) {
  const trimmed = value.trim()
  try {
    return decodeURIComponent(trimmed)
  } catch {
    return trimmed
  }
}

function normalizeRequestError(error: unknown, callerAborted: boolean) {
  if (error instanceof DOMException && error.name === 'AbortError') {
    return new ApiError(callerAborted ? '请求已取消。' : '请求超时。', 0)
  }

  if (error instanceof Error) {
    return error
  }

  return new ApiError('请求失败。', 0)
}

function isNetworkUnavailableError(error: unknown, callerAborted: boolean) {
  if (callerAborted) {
    return false
  }

  if (error instanceof DOMException && error.name === 'AbortError') {
    return false
  }

  if (error instanceof TypeError) {
    return true
  }

  return error instanceof Error && /failed to fetch|network|load failed/i.test(error.message)
}

async function readResponsePayload(response: Response) {
  if (response.status === 204) {
    return undefined
  }

  const contentType = response.headers.get('content-type') ?? ''
  const isJson = contentType.includes('application/json')
  return isJson ? await response.json() : await response.text()
}

function readErrorEnvelope(payload: unknown) {
  return typeof payload === 'object' && payload !== null && 'error' in payload
    ? (payload as ErrorEnvelope)
    : undefined
}

function createApiError(response: Response, payload: unknown) {
  const errorEnvelope = readErrorEnvelope(payload)

  return new ApiError(
    errorEnvelope?.error.message
      ?? (typeof payload === 'string' && payload.trim() ? payload.trim() : response.statusText),
    response.status,
    errorEnvelope?.error.code,
    errorEnvelope?.error.request_id,
    errorEnvelope?.error.details,
    errorEnvelope?.error.message_key,
  )
}

function isBackendUnavailableResponse(response: Response) {
  return response.status === 503 && response.headers.get(backendUnavailableHeader) === '1'
}

export async function apiRequest<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const { auth = true, headers, body, timeoutMs = DEFAULT_TIMEOUT_MS, signal: callerSignal, acceptStatuses = [], ...rest } = options
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
  } catch (error) {
    const normalizedError = normalizeRequestError(error, Boolean(callerSignal?.aborted))
    if (isNetworkUnavailableError(error, Boolean(callerSignal?.aborted))) {
      runtime.onNetworkUnavailable(path, normalizedError)
    }
    throw normalizedError
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId)
  }

  const payload = await readResponsePayload(response)

  if (isBackendUnavailableResponse(response)) {
    const error = createApiError(response, payload)
    runtime.onNetworkUnavailable(path, error)
    throw error
  }

  runtime.onReachable(path, response.status)

  if (!response.ok && !acceptStatuses.includes(response.status)) {
    if (response.status === 401 && auth) {
      runtime.onUnauthorized(tokenSnapshot)
    }

    throw createApiError(response, payload)
  }

  if (response.status === 204) {
    return undefined as T
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
  } catch (error) {
    const normalizedError = normalizeRequestError(error, Boolean(callerSignal?.aborted))
    if (isNetworkUnavailableError(error, Boolean(callerSignal?.aborted))) {
      runtime.onNetworkUnavailable(path, normalizedError)
    }
    throw normalizedError
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId)
  }

  if (!response.ok) {
    const payload = await readResponsePayload(response)

    if (isBackendUnavailableResponse(response)) {
      const error = createApiError(response, payload)
      runtime.onNetworkUnavailable(path, error)
      throw error
    }

    runtime.onReachable(path, response.status)

    if (response.status === 401 && auth) {
      runtime.onUnauthorized(tokenSnapshot)
    }

    throw createApiError(response, payload)
  }

  runtime.onReachable(path, response.status)

  return {
    blob: await response.blob(),
    filename: parseDownloadFilename(response.headers.get('content-disposition')),
  }
}
