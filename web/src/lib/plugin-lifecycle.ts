import { t } from '@/i18n'
import type { PluginState } from '@/types/api'

// Reload restarts the runtime of an enabled plugin. A disabled plugin has none, a starting or stopping one is busy,
// and an invalid manifest cannot load at all, so each gets its own reason where the reload control is disabled.
export function getPluginReloadBlocker(state?: PluginState | string): string | null {
  switch (state) {
    case 'disabled':
      return t('plugins.actions.reloadUnavailable')
    case 'starting':
    case 'stopping':
      return t('plugins.actions.reloadAfterSwitch')
    case 'invalid':
      return t('plugins.actions.reloadInvalid')
    default:
      return null
  }
}

// The server does not report whether an invalid plugin is meant to run, so its switch could only guess.
export function canTogglePluginPower(state?: PluginState | string) {
  return state !== 'invalid'
}
