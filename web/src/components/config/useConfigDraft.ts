import { computed, onActivated, onDeactivated, onScopeDispose, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess } from '@/adapter/feedback'
import { cloneConfig, getValueByPath, setValueByPath, type ConfigFieldDefinition } from '@/lib/config-form'
import { fromMultilineList, toMultilineList } from '@/lib/format'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

export function useConfigDraft() {
  const store = useConfigStore()
  const { document, saving } = storeToRefs(store)
  const draft = ref<ConfigDocument | null>(null)
  const saveStatus = ref<'hot' | 'restart' | null>(null)
  let active = true
  let saveStatusTimer: ReturnType<typeof setTimeout> | undefined

  watch(document, (value, previous) => {
    if (value && previous && JSON.stringify(value) === JSON.stringify(previous)) return
    draft.value = value ? cloneConfig(value) : null
  }, { immediate: true })

  const hasUnsavedChanges = computed(() => Boolean(draft.value && document.value)
    && JSON.stringify(draft.value) !== JSON.stringify(document.value))
  const canSave = computed(() => hasUnsavedChanges.value && !saving.value)

  function markDraftChanged() {
    clearTimeout(saveStatusTimer)
    saveStatusTimer = undefined
    saveStatus.value = null
  }

  onActivated(() => { active = true })
  const deactivate = () => { active = false; markDraftChanged() }
  onDeactivated(deactivate)
  onScopeDispose(deactivate)

  function readField(path: string, type: ConfigFieldDefinition['type']) {
    if (!draft.value) return type === 'boolean' ? false : type === 'number' ? null : ''
    const value = getValueByPath(draft.value as unknown as Record<string, unknown>, path)
    return type === 'list' ? (Array.isArray(value) ? toMultilineList(value as string[]) : '') : value
  }

  function writeField(path: string, type: ConfigFieldDefinition['type'], value: unknown) {
    if (!draft.value) return
    let normalized = value
    if (type === 'number') {
      const number = value === null || value === undefined || value === '' ? NaN : Number(value)
      normalized = Number.isFinite(number) ? number : undefined
    } else if (type === 'list') {
      normalized = Array.isArray(value) ? value : fromMultilineList(String(value))
    }
    markDraftChanged()
    setValueByPath(draft.value as unknown as Record<string, unknown>, path, normalized)
  }

  async function save() {
    if (!draft.value || !canSave.value) return
    const response = await store.saveConfig(draft.value)
    markDraftChanged()
    if (active) {
      saveStatus.value = response.restart_required ? 'restart' : 'hot'
      saveStatusTimer = setTimeout(markDraftChanged, 3000)
    }
    notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
    return response
  }

  return { draft, saveStatus, hasUnsavedChanges, canSave, markDraftChanged, readField, writeField, save }
}
