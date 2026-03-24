import type { ErrorEnvelope } from '@/types/api'

export class ApiError extends Error {
  code: string
  status: number
  requestId?: string
  details?: Record<string, unknown>

  constructor(message: string, status: number, code = 'platform.unknown', requestId?: string, details?: Record<string, unknown>) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.requestId = requestId
    this.details = details
  }
}

interface RuntimeConfig {
  getToken: () => string | null
  onUnauthorized: () => void
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
}

export interface ApiDownloadResult {
  blob: Blob
  filename: string | null
}

function withRuntimeHeaders(headers: HeadersInit | undefined, auth: boolean, hasBody: boolean) {
  const requestHeaders = new Headers(headers)

  if (hasBody) {
    requestHeaders.set('Content-Type', 'application/json')
  }

  if (auth) {
    const token = runtime.getToken()
    if (token) {
      requestHeaders.set('Authorization', `Bearer ${token}`)
    }
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
  const { auth = true, headers, body, ...rest } = options
  const requestHeaders = withRuntimeHeaders(headers, auth, body !== undefined)

  const response = await fetch(path, {
    ...rest,
    headers: requestHeaders,
    body: body === undefined ? undefined : JSON.stringify(body),
  })

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
      runtime.onUnauthorized()
    }

    throw new ApiError(
      errorEnvelope?.error.message ?? response.statusText,
      response.status,
      errorEnvelope?.error.code,
      errorEnvelope?.error.request_id,
      errorEnvelope?.error.details,
    )
  }

  return payload as T
}

export async function apiDownload(path: string, options: ApiRequestOptions = {}): Promise<ApiDownloadResult> {
  const { auth = true, headers, body, ...rest } = options
  const requestHeaders = withRuntimeHeaders(headers, auth, body !== undefined)

  const response = await fetch(path, {
    ...rest,
    headers: requestHeaders,
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  if (!response.ok) {
    const contentType = response.headers.get('content-type') ?? ''
    const isJson = contentType.includes('application/json')
    const payload = isJson ? await response.json() : await response.text()
    const errorEnvelope = typeof payload === 'object' && payload !== null && 'error' in payload
      ? (payload as ErrorEnvelope)
      : undefined

    if (response.status === 401 && auth) {
      runtime.onUnauthorized()
    }

    throw new ApiError(
      errorEnvelope?.error.message ?? response.statusText,
      response.status,
      errorEnvelope?.error.code,
      errorEnvelope?.error.request_id,
      errorEnvelope?.error.details,
    )
  }

  return {
    blob: await response.blob(),
    filename: parseDownloadFilename(response.headers.get('content-disposition')),
  }
}
