import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import {
  cloneConfig,
  getProtocolConfigSections,
  getValueByPath,
  setValueByPath,
  type ConfigFieldDefinition,
} from '@/lib/config-form'
import { fromMultilineList, toMultilineList } from '@/lib/format'
import { useConfigStore } from '@/stores/config'
import { useProtocolsStore } from '@/stores/protocols'
import type { ConfigDocument } from '@/types/api'

export function useProtocolConfigEditor(
  configStore: ReturnType<typeof useConfigStore>,
  protocolsStore: ReturnType<typeof useProtocolsStore>,
) {
  const { document, saving } = storeToRefs(configStore)
  const draft = ref<ConfigDocument | null>(null)
  const advancedExpanded = ref(false)
  const configSections = computed(() => getProtocolConfigSections())

  watch(document, (value) => {
    draft.value = value ? cloneConfig(value) : null
  }, { immediate: true })

  function readField(path: string, type: ConfigFieldDefinition['type']) {
    if (!draft.value) {
      if (type === 'boolean') {
        return false
      }

      return type === 'number' ? null : ''
    }

    const current = getValueByPath(draft.value as unknown as Record<string, unknown>, path)
    if (type === 'list') {
      return Array.isArray(current) ? toMultilineList(current as string[]) : ''
    }
    return current
  }

  function writeField(path: string, type: ConfigFieldDefinition['type'], value: unknown) {
    if (!draft.value) {
      return
    }

    let normalized = value
    if (type === 'number') {
      if (value === null || value === undefined || value === '') {
        normalized = undefined
      } else {
        const nextNumber = Number(value)
        normalized = Number.isFinite(nextNumber) ? nextNumber : undefined
      }
    } else if (type === 'list') {
      normalized = fromMultilineList(String(value))
    }

    setValueByPath(draft.value as unknown as Record<string, unknown>, path, normalized)
  }

  const canSave = computed(() => Boolean(draft.value) && !saving.value)

  async function save() {
    if (!draft.value) {
      return
    }

    const response = await configStore.saveConfig(draft.value)
    try {
      await protocolsStore.refresh()
    } catch {
      // store error state drives the page
    }
    notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
  }

  return {
    advancedExpanded,
    canSave,
    configSections,
    draft,
    readField,
    save,
    writeField,
  }
}
