<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppAlert from '@/components/AppAlert.vue'
import AppButton from '@/components/AppButton.vue'
import AppCheckbox from '@/components/AppCheckbox.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppField from '@/components/AppField.vue'
import AppInput from '@/components/AppInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginInstallSourceType } from '@/types/api'

const open = defineModel<boolean>('open', { required: true })

const pluginsStore = usePluginsStore()
const { installPending } = storeToRefs(pluginsStore)
const installError = ref<string | null>(null)
const installForm = reactive<{ source_type: PluginInstallSourceType; source: string }>({
  source_type: 'local_zip',
  source: '',
})
const trustedCodeConfirmed = ref(false)

// While the dialog is open the error sits beside the field; a failure after it closed still arrives as a toast.
useToastFeedback(computed(() => (
  installError.value && !open.value
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
    // The dialog closes as soon as the server accepts the task; the result arrives as a toast.
    await pluginsStore.installPlugin({
      source_type: installForm.source_type,
      source: installForm.source.trim(),
      trusted_code_confirmed: true,
    }, () => { open.value = false })
    open.value = false
    notifySuccess(t('plugins.installAccepted'))
  } catch (error) {
    installError.value = getDisplayErrorMessage(error)
  }
}

function resetInstallDialog() {
  installForm.source_type = 'local_zip'
  installForm.source = ''
  trustedCodeConfirmed.value = false
  if (!installPending.value) installError.value = null
}
</script>

<template>
  <AppDialog
    :open="open"
    :title="t('plugins.installDialogTitle')"
    :busy="installPending"
    @close="open = false"
    @after-close="resetInstallDialog"
  >
    <div>
      <AppField floating :label="t('plugins.sourceType')">
        <AppSelect
          v-model="installForm.source_type"
          :options="[
            { label: t('plugins.localZip'), value: 'local_zip' },
            { label: t('plugins.localDirectory'), value: 'local_directory' },
            { label: t('plugins.remoteUrl'), value: 'remote_url' },
          ]"
        />
      </AppField>

      <AppField
        floating
        :label="installForm.source_type === 'remote_url' ? t('plugins.remoteUrlLabel') : t('plugins.serverPath')"
        :hint="installForm.source_type === 'remote_url' ? t('plugins.remoteUrlHint') : t('plugins.serverPathHint')"
        :error="installError ?? undefined"
      >
        <AppInput v-model="installForm.source" @update:model-value="installError = null" />
      </AppField>

      <AppAlert
        tone="attention"
        :title="t('plugins.installTrust.title')"
        :description="t('plugins.installTrust.description')"
      />

      <AppCheckbox v-model="trustedCodeConfirmed">
        {{ t('plugins.installTrust.confirm') }}
      </AppCheckbox>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <AppButton :disabled="installPending" @click="open = false">{{ t('dashboard.previewCancel') }}</AppButton>
        <AppButton variant="default" :loading="installPending" :disabled="!installForm.source.trim() || !trustedCodeConfirmed" @click="submitInstall">{{ t('plugins.installSubmit') }}</AppButton>
      </div>
    </template>
  </AppDialog>
</template>
