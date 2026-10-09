import { i18n, t } from '@/i18n'
import { formatPluginVersion } from '@/lib/display'
import { ApiError } from '@/lib/http'

// Browser-side failures have no server error code; each client code maps to its own message.
const clientErrorKeys: Record<string, string> = {
  'client.request_cancelled': 'errors.common.requestCancelled',
  'client.request_timeout': 'errors.common.requestTimeout',
  'client.task_cancelled': 'errors.common.taskCancelled',
  'client.task_interrupted': 'errors.common.taskInterrupted',
  'client.task_still_running': 'errors.common.taskStillRunning',
}

function translateErrorCode(code: string | undefined) {
  if (!code) return undefined
  const localeKey = `errors.${code}`
  return i18n.global.te(localeKey) ? t(localeKey) : undefined
}

export function getErrorCodeMessage(code: string | undefined, fallbackKey = 'errors.common.actionFailed') {
  return translateErrorCode(code) ?? t(fallbackKey)
}

// An install refused for RayleaBot's version says in its details whether that version is unknown or too old, and the
// two need different remedies.
function describeCoreVersionIncompatible(details?: Record<string, unknown>) {
  if (details?.incompatible_reason === 'core_version_unknown') return t('errors.coreVersion.unknown')
  if (details?.incompatible_reason === 'core_version_too_old' && typeof details.min_core_version === 'string') {
    return t('errors.coreVersion.tooOld', { version: formatPluginVersion(details.min_core_version) })
  }
  return undefined
}

// A refused install lists the required plugins that are still missing, so the message can name them.
function describeDependencyMissing(details?: Record<string, unknown>) {
  const ids = Array.isArray(details?.plugin_ids) ? details.plugin_ids.filter((id): id is string => typeof id === 'string') : []
  return ids.length > 0 ? t('errors.dependencyMissing', { plugins: ids.join('、') }) : undefined
}

const detailedErrors: Record<string, (details?: Record<string, unknown>) => string | undefined> = {
  'plugin.core_version_incompatible': describeCoreVersionIncompatible,
  'plugin.dependency_missing': describeDependencyMissing,
}

export function getDisplayErrorMessage(error: unknown, fallbackKey = 'errors.common.actionFailed') {
  if (error instanceof ApiError) {
    const clientKey = clientErrorKeys[error.code]
    if (clientKey) return t(clientKey)
    const specific = detailedErrors[error.code]?.(error.details)
    if (specific) return specific
    const message = translateErrorCode(error.code)
    if (message) return message
  }
  return t(fallbackKey)
}
