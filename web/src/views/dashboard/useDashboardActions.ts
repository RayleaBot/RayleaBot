import { notifyError, notifySuccess } from '@/adapter/feedback'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import type { RuntimeBootstrapResource } from '@/types/api'
type DashboardActionState = {
  systemStore: {
    createBackup: () => Promise<{ task_id: string }>
    exportDiagnostics: () => Promise<void>
    bootstrapManagedRuntime: (resources: RuntimeBootstrapResource[]) => Promise<{ task_id: string }>
  }
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

  async function bootstrapRuntimeResources(resources: RuntimeBootstrapResource[]) {
    if (resources.length === 0) return
    try {
      await state.systemStore.bootstrapManagedRuntime(resources)
      notifySuccess(t('dashboard.runtimeBootstrapAccepted'))
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  return {
    bootstrapRuntimeResources,
    createBackup,
    exportDiagnostics,
  }
}
