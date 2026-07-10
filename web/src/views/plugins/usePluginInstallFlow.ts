import { computed, reactive, ref, watch } from 'vue'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginInstallInspectionRequest, PluginInstallInspectionResponse } from '@/types/api'

export function usePluginInstallFlow(pluginsStore: ReturnType<typeof usePluginsStore>) {
  const installDialogVisible = ref(false)
  const installError = ref<string | null>(null)
  const installForm = reactive<PluginInstallInspectionRequest>({
    source_type: 'local_zip',
    source: '',
  })
  const installInspection = ref<PluginInstallInspectionResponse | null>(null)
  const trustedCodeConfirmed = ref(false)
  const allowInstallScripts = ref(false)

  watch(
    () => [installForm.source_type, installForm.source] as const,
    () => {
      installInspection.value = null
      trustedCodeConfirmed.value = false
      allowInstallScripts.value = false
    },
  )

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
    try {
      if (!installInspection.value) {
        installInspection.value = await pluginsStore.inspectPlugin({
          source_type: installForm.source_type,
          source: installForm.source.trim(),
        })
        return
      }
      if (!trustedCodeConfirmed.value) {
        installError.value = '请确认该插件及获准的安装脚本将作为完全可信的本地代码运行。'
        return
      }
      if (installInspection.value.install_scripts.length > 0 && !allowInstallScripts.value) {
        installError.value = '该插件声明了安装脚本。请明确授权安装脚本后再继续。'
        return
      }
      await pluginsStore.installPlugin({
        source_type: installInspection.value.source.source_type,
        source: installInspection.value.source.source,
        inspection_id: installInspection.value.inspection_id,
        package_sha256: installInspection.value.package_sha256,
        trusted_code_confirmed: true,
        allow_install_scripts: allowInstallScripts.value,
      })
      installDialogVisible.value = false
      resetInstallDialog()
      notifySuccess(t('plugins.installAccepted'))
    } catch (error) {
      installError.value = getDisplayErrorMessage(error)
    }
  }

  function resetInstallDialog() {
    installForm.source_type = 'local_zip'
    installForm.source = ''
    installInspection.value = null
    trustedCodeConfirmed.value = false
    allowInstallScripts.value = false
  }

  return {
    allowInstallScripts,
    installDialogVisible,
    installForm,
    installInspection,
    resetInstallDialog,
    submitInstall,
    trustedCodeConfirmed,
  }
}
