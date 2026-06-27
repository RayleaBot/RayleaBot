import type { ComputedRef, Ref } from 'vue'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { buildPluginDetailLocation } from '@/lib/management-links'
import { t } from '@/i18n'
type DashboardActionState = {
  systemStore: {
    createBackup: () => Promise<{ task_id: string }>
    exportDiagnostics: () => Promise<void>
    recheckRecovery: () => Promise<{ task_id: string }>
    confirmRecovery: (payload: { review_ids: string[]; note?: string }) => Promise<{ task_id: string }>
    bootstrapManagedRuntime: (resources: string[]) => Promise<{ task_id: string }>
  }
  router: {
    push: (location: unknown) => Promise<unknown>
  }
  recoveryBootstrapResources: ComputedRef<string[]>
  recoveryConfirmNote: Ref<string>
  selectedRecoveryReviewIds: Ref<string[]>
}

export function useDashboardActions(state: DashboardActionState) {
  async function createBackup() {
    try {
      await state.systemStore.createBackup()
      notifySuccess(t('dashboard.backupAccepted'))
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

  async function recheckRecoverySummary() {
    try {
      await state.systemStore.recheckRecovery()
      notifySuccess(t('dashboard.recoveryRecheckAccepted'))
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function confirmRecoverySelection() {
    if (state.selectedRecoveryReviewIds.value.length === 0) return

    try {
      await state.systemStore.confirmRecovery({
        review_ids: [...state.selectedRecoveryReviewIds.value],
        note: state.recoveryConfirmNote.value.trim() || undefined,
      })
      notifySuccess(t('dashboard.recoveryConfirmAccepted'))
      state.selectedRecoveryReviewIds.value = []
      state.recoveryConfirmNote.value = ''
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function bootstrapRuntimeResources() {
    try {
      await state.systemStore.bootstrapManagedRuntime(state.recoveryBootstrapResources.value)
      notifySuccess(t('dashboard.runtimeBootstrapAccepted'))
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  async function openRecoveryPlugin(pluginID: string) {
    await state.router.push(buildPluginDetailLocation(pluginID))
  }

  return {
    bootstrapRuntimeResources,
    confirmRecoverySelection,
    createBackup,
    exportDiagnostics,
    openRecoveryPlugin,
    recheckRecoverySummary,
  }
}
