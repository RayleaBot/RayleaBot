import { computed, type Ref } from 'vue'
import { t } from '@/i18n'
import { type LogFilters } from '@/stores/log-state'
import type { LogLevel } from '@/types/api'

export function useLogFilterControls(filters: Ref<LogFilters>) {
  const selectedLevels = computed({
    get: () => filters.value.levels ?? [],
    set: (levels: LogLevel[]) => { filters.value.levels = levels },
  })
  const levelOptions = computed(() => ([
    { label: t('display.logLevels.debug'), value: 'debug' as LogLevel },
    { label: t('display.logLevels.info'), value: 'info' as LogLevel },
    { label: t('display.logLevels.warn'), value: 'warn' as LogLevel },
    { label: t('display.logLevels.error'), value: 'error' as LogLevel },
  ]))

  return { selectedLevels, levelOptions }
}
