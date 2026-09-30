import { computed, type Ref } from 'vue'
import { t } from '@/i18n'
import { type LogFilters } from '@/stores/log-state'
import type { LogLevel, LogProtocol } from '@/types/api'

// The filters after level and source, in toolbar order; when the row runs out of room they give way to
// 更多筛选 from the end. Their toolbar widths are fixed so the fit can be worked out before they are laid out.
export const logAdvancedFilterKeys = ['protocol', 'plugin', 'requestId'] as const
export type LogAdvancedFilterKey = typeof logAdvancedFilterKeys[number]
export const logAdvancedFilterWidths: Record<LogAdvancedFilterKey, number> = { protocol: 150, plugin: 220, requestId: 240 }

export function isLogAdvancedFilterActive(filters: LogFilters, key: LogAdvancedFilterKey) {
  if (key === 'protocol') return Boolean(filters.protocol)
  if (key === 'plugin') return Boolean(filters.pluginIds?.length)
  return Boolean(filters.requestId?.trim())
}

// How many optional items, taken in order, fit on one row after the fixed ones. The overflow trigger only
// takes room while something is left over, so a row that holds every item shows no trigger.
export function fitInlineFilterCount(input: {
  rowWidth: number
  fixedWidth: number
  optionalWidths: readonly number[]
  overflowWidth: number
  gap: number
}) {
  const { rowWidth, fixedWidth, optionalWidths, overflowWidth, gap } = input
  const allWidth = optionalWidths.reduce((sum, width) => sum + gap + width, fixedWidth)
  if (allWidth <= rowWidth) return optionalWidths.length
  let used = fixedWidth + gap + overflowWidth
  let count = 0
  for (const width of optionalWidths) {
    if (used + gap + width > rowWidth) break
    used += gap + width
    count += 1
  }
  return count
}

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
  const protocolOptions = computed(() => ([
    { value: '', label: t('logs.filters.all') },
    { value: 'onebot11', label: t('display.logProtocols.onebot11') },
    { value: 'qqofficial', label: t('display.logProtocols.qqofficial') },
  ]))
  const protocol = computed({
    get: () => filters.value.protocol ?? '',
    set: (value: string) => { filters.value.protocol = (value || undefined) as LogProtocol | undefined },
  })
  const pluginIds = computed({
    get: () => filters.value.pluginIds ?? [],
    set: (ids: string[]) => { filters.value.pluginIds = ids },
  })
  const requestId = computed({
    get: () => filters.value.requestId ?? '',
    set: (value: string) => { filters.value.requestId = value },
  })

  return { selectedLevels, levelOptions, protocolOptions, protocol, pluginIds, requestId }
}
