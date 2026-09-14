import { computed, reactive, ref } from 'vue'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginInstallSourceType } from '@/types/api'

export function usePluginInstallFlow(pluginsStore: ReturnType<typeof usePluginsStore>) {
  const installDialogVisible = ref(false)
  const installError = ref<string | null>(null)
  const installForm = reactive<{ source_type: PluginInstallSourceType; source: string }>({
    source_type: 'local_zip',
    source: '',
  })
  const trustedCodeConfirmed = ref(false)

  useToastFeedback(computed(() => (
    installError.value
      ? {
          key: `plugins-install-error:${installError.value}`,
          level: 'error' as const,
          message: installError.value,
        }
      : null
  )))

  async function submitInstall() {
    installError.value = null
    if (!trustedCodeConfirmed.value) {
      installError.value = t('plugins.installTrust.required')
      return
    }
    try {
      await pluginsStore.installPlugin({
        source_type: installForm.source_type,
        source: installForm.source.trim(),
        trusted_code_confirmed: true,
      }, () => { installDialogVisible.value = false })
      installDialogVisible.value = false
      notifySuccess(t('plugins.installAccepted'))
    } catch (error) {
      installError.value = getDisplayErrorMessage(error)
    }
  }

  function resetInstallDialog() {
    installForm.source_type = 'local_zip'
    installForm.source = ''
    trustedCodeConfirmed.value = false
  }

  return {
    installDialogVisible,
    installForm,
    resetInstallDialog,
    submitInstall,
    trustedCodeConfirmed,
  }
}
