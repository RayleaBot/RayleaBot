import { t } from '@/i18n'
import type { ApiPath } from '@/lib/api-path'
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
  getCSRFToken: () => string | null
  onCSRFToken: (token: string) => void
  onNetworkUnavailable: (path: string, error: Error) => void
  onReachable: (path: string, status: number) => void
  onUnauthorized: () => void
}

const runtime: RuntimeConfig = {
  getCSRFToken: () => null,
  onCSRFToken: () => undefined,
  onNetworkUnavailable: () => undefined,
  onReachable: () => undefined,
  onUnauthorized: () => undefined,
}

export function configureApiRuntime(config: Partial<RuntimeConfig>) {
  if (config.getCSRFToken) {
    runtime.getCSRFToken = config.getCSRFToken
  }

  if (config.onCSRFToken) {
    runtime.onCSRFToken = config.onCSRFToken
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
  method: string,
) {
  const requestHeaders = new Headers(headers)

  if (hasBody) {
    requestHeaders.set('Content-Type', 'application/json')
  }

  const csrfToken = auth && !['GET', 'HEAD', 'OPTIONS', 'TRACE'].includes(method.toUpperCase())
    ? runtime.getCSRFToken()
    : null
  if (csrfToken) {
    requestHeaders.set('X-Raylea-CSRF', csrfToken)
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

function requestAborted(cause: 'cancelled' | 'timeout') {
  return new ApiError(t(cause === 'cancelled' ? 'errors.common.requestCancelled' : 'errors.common.requestTimeout'), 0, `client.request_${cause}`)
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
  if (typeof payload !== 'object' || payload === null || !('error' in payload)) return undefined
  const error = payload.error
  return typeof error === 'object' && error !== null ? payload as ErrorEnvelope : undefined
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

async function executeRequest<T>(
  path: string,
  options: ApiRequestOptions,
  readSuccess: (response: Response) => Promise<T>,
): Promise<T> {
  const { auth = true, headers, body, timeoutMs = DEFAULT_TIMEOUT_MS, signal: callerSignal, acceptStatuses = [], ...rest } = options
  if (callerSignal?.aborted) throw requestAborted('cancelled')
  const requestHeaders = withRuntimeHeaders(headers, auth, body !== undefined, rest.method ?? 'GET')
  const requestBody = body === undefined ? undefined : JSON.stringify(body)
  const controller = new AbortController()
  let abortCause: 'cancelled' | 'timeout' | undefined
  const abort = (cause: 'cancelled' | 'timeout') => {
    if (controller.signal.aborted) return
    abortCause = cause
    controller.abort()
  }
  const onCallerAbort = () => abort('cancelled')
  callerSignal?.addEventListener('abort', onCallerAbort, { once: true })
  const timeoutId = timeoutMs > 0 ? setTimeout(() => abort('timeout'), timeoutMs) : undefined
  try {
    const response = await fetch(path, {
      ...rest,
      credentials: 'same-origin',
      signal: controller.signal,
      headers: requestHeaders,
      body: requestBody,
    })
    if (abortCause) throw requestAborted(abortCause)
    const nextCSRFToken = response.headers.get('X-Raylea-CSRF')?.trim()
    if (nextCSRFToken) runtime.onCSRFToken(nextCSRFToken)

    if (isBackendUnavailableResponse(response)) {
      const error = createApiError(response, await readResponsePayload(response))
      runtime.onNetworkUnavailable(path, error)
      throw error
    }
    runtime.onReachable(path, response.status)
    if (!response.ok && !acceptStatuses.includes(response.status)) {
      if (response.status === 401 && auth) runtime.onUnauthorized()
      throw createApiError(response, await readResponsePayload(response))
    }
    const result = await readSuccess(response)
    if (abortCause) throw requestAborted(abortCause)
    return result
  } catch (error) {
    if (abortCause) throw requestAborted(abortCause)
    if (error instanceof DOMException && ['AbortError', 'TimeoutError'].includes(error.name)) {
      throw requestAborted(error.name === 'TimeoutError' ? 'timeout' : 'cancelled')
    }
    if (error instanceof TypeError) runtime.onNetworkUnavailable(path, error)
    throw error instanceof Error ? error : new ApiError(t('errors.common.actionFailed'), 0)
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId)
    callerSignal?.removeEventListener('abort', onCallerAbort)
  }
}

export async function apiRequest<T>(path: ApiPath, options: ApiRequestOptions = {}): Promise<T> {
  return executeRequest(path, options, response => readResponsePayload(response) as Promise<T>)
}

export async function apiDownload(path: ApiPath, options: ApiRequestOptions = {}): Promise<ApiDownloadResult> {
  return executeRequest(path, options, async response => ({
    blob: await response.blob(),
    filename: parseDownloadFilename(response.headers.get('content-disposition')),
  }))
}
