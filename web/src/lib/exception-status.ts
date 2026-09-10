import { ApiError } from '@/lib/http'

export type ExceptionStatus = '403' | '404' | '500'

export function resolveExceptionStatus(error: unknown, fallback: ExceptionStatus = '500'): ExceptionStatus {
  if (!(error instanceof ApiError)) return fallback
  if (error.code === 'permission.denied' || error.status === 403) return '403'
  if (error.code === 'platform.resource_not_found' || error.status === 404) return '404'
  return fallback
}
