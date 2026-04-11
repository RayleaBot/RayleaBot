import type { ComputedRef, Ref } from 'vue'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
type DashboardActionState = {
  systemStore: {
    createBackup: () => Promise<{ task_id: string }>
    exportDiagnostics: () => Promise<void>
    previewRender: (payload: {
      template: string
      theme?: string
      output: 'png' | 'jpeg'
      data: Record<string, unknown>
    }) => Promise<{ task_id: string }>
    recheckRecovery: () => Promise<{ task_id: string }>
    confirmRecovery: (payload: { review_ids: string[]; note?: string }) => Promise<{ task_id: string }>
    bootstrapManagedRuntime: (resources: string[]) => Promise<{ task_id: string }>
  }
  router: {
    push: (location: unknown) => Promise<unknown>
  }
  previewForm: {
    template: string
    theme: string
    output: 'png' | 'jpeg'
    dataText: string
  }
  previewVisible: Ref<boolean>
  recoveryBootstrapResources: ComputedRef<string[]>
  recoveryConfirmNote: Ref<string>
  selectedRecoveryReviewIds: Ref<string[]>
}

export function useDashboardActions(state: DashboardActionState) {
  async function createBackup() {
    try {
      const response = await state.systemStore.createBackup()
      notifySuccess(t('dashboard.backupAccepted'))
      await state.router.push({ name: 'tasks', query: { task_id: response.task_id } })
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function exportDiagnostics() {
    try {
      await state.systemStore.exportDiagnostics()
      notifySuccess(t('dashboard.diagnosticsAccepted'))
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function submitRenderPreview() {
    let data: Record<string, unknown>
    try {
      const parsed = JSON.parse(state.previewForm.dataText)
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
        throw new Error(t('errors.platform.invalidRequest'))
      }
      data = parsed as Record<string, unknown>
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
      return
    }

    try {
      const response = await state.systemStore.previewRender({
        template: state.previewForm.template,
        theme: state.previewForm.theme || undefined,
        output: state.previewForm.output,
        data,
      })
      state.previewVisible.value = false
      notifySuccess(t('dashboard.previewAccepted'))
      await state.router.push({ name: 'tasks', query: { task_id: response.task_id } })
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function recheckRecoverySummary() {
    try {
      const response = await state.systemStore.recheckRecovery()
      notifySuccess(t('dashboard.recoveryRecheckAccepted'))
      await state.router.push({ name: 'tasks', query: { task_id: response.task_id } })
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function confirmRecoverySelection() {
    if (state.selectedRecoveryReviewIds.value.length === 0) return

    try {
      const response = await state.systemStore.confirmRecovery({
        review_ids: [...state.selectedRecoveryReviewIds.value],
        note: state.recoveryConfirmNote.value.trim() || undefined,
      })
      notifySuccess(t('dashboard.recoveryConfirmAccepted'))
      state.selectedRecoveryReviewIds.value = []
      state.recoveryConfirmNote.value = ''
      await state.router.push({ name: 'tasks', query: { task_id: response.task_id } })
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function bootstrapRuntimeResources() {
    try {
      const response = await state.systemStore.bootstrapManagedRuntime(state.recoveryBootstrapResources.value)
      notifySuccess(t('dashboard.runtimeBootstrapAccepted'))
      await state.router.push({ name: 'tasks', query: { task_id: response.task_id } })
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function openRecoveryPlugin(pluginID: string) {
    await state.router.push({ name: 'plugin-detail', params: { id: pluginID } })
  }

  return {
    bootstrapRuntimeResources,
    confirmRecoverySelection,
    createBackup,
    exportDiagnostics,
    openRecoveryPlugin,
    recheckRecoverySummary,
    submitRenderPreview,
  }
}
