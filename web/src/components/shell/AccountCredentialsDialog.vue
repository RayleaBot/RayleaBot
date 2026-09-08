<script setup lang="ts">
import { nextTick, ref, useId } from 'vue'
import { AnimatePresence, motion } from 'motion-v'
import { ChevronDownIcon, LockKeyholeIcon, UserRoundPenIcon } from '@lucide/vue'
import AppButton from '@/components/AppButton.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppField from '@/components/AppField.vue'
import AppInput from '@/components/AppInput.vue'
import { t } from '@/i18n'
import { ApiError } from '@/lib/http'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useOverlayMotion } from '@/motion/presets'
import { useSessionStore } from '@/stores/session'
import { notifySuccess } from '@/adapter/feedback'

defineProps<{ open: boolean; fallbackFocus?: string }>()
const emit = defineEmits<{ close: [] }>()
const session = useSessionStore()
const formId = useId()
const usernameId = `${formId}-username`
const form = ref<HTMLFormElement | null>(null)
const currentSecret = ref('')
const newSecret = ref('')
const confirmSecret = ref('')
const newIdentifier = ref('')
const showUsername = ref(false)
const pending = ref(false)
const saved = ref(false)
const errors = ref<Record<string, string>>({})
const feedback = ref('')
const overlayMotion = useOverlayMotion()

async function focusFirstError() {
  await nextTick()
  form.value?.querySelector<HTMLInputElement>('[aria-invalid=true]')?.focus()
}

async function submit() {
  if (pending.value || saved.value) return
  errors.value = {}
  feedback.value = ''
  if (!currentSecret.value) errors.value.current = t('shell.credentials.currentRequired')
  const passwordLength = Array.from(newSecret.value).length
  if (passwordLength < 8 || passwordLength > 1024) errors.value.next = t('shell.credentials.passwordLength')
  if (!confirmSecret.value || confirmSecret.value !== newSecret.value) errors.value.confirm = t('shell.credentials.passwordMismatch')
  if (showUsername.value && Array.from(newIdentifier.value).length > 128) errors.value.identifier = t('shell.credentials.usernameLength')
  if (Object.keys(errors.value).length) {
    await focusFirstError()
    return
  }

  pending.value = true
  try {
    await session.updateCredentials({
      current_secret: currentSecret.value,
      new_secret: newSecret.value,
      ...(showUsername.value && newIdentifier.value.trim() ? { new_identifier: newIdentifier.value } : {}),
    })
    saved.value = true
    // A session_expired frame can unmount this dialog before the HTTP response.
    // Keep the completion feedback independent of the layout's event listener.
    notifySuccess(t('shell.credentials.saved'))
    emit('close')
  } catch (error) {
    if (error instanceof ApiError && error.code === 'permission.current_secret_invalid') {
      errors.value.current = getDisplayErrorMessage(error)
    } else if (error instanceof ApiError && error.code === 'platform.rate_limited') {
      feedback.value = t('shell.credentials.rateLimited')
    } else {
      feedback.value = getDisplayErrorMessage(error)
    }
  } finally {
    pending.value = false
  }
  if (Object.keys(errors.value).length) await focusFirstError()
}

function afterClose() {
  currentSecret.value = newSecret.value = confirmSecret.value = newIdentifier.value = ''
  showUsername.value = saved.value = false
  errors.value = {}
  feedback.value = ''
}
</script>

<template>
  <AppDialog :open="open" :title="t('shell.credentials.title')" :description="t('shell.credentials.description')" :width="480" :busy="pending" :fallback-focus="fallbackFocus" initial-focus="[data-account-current-password]" data-testid="account-credentials-dialog" @close="$emit('close')" @after-close="afterClose">
    <form :id="formId" ref="form" class="account-credentials" novalidate @submit.prevent="submit">
      <AppField floating :label="t('shell.credentials.currentPassword')" :error="errors.current" required>
        <AppInput v-model="currentSecret" type="password" autocomplete="current-password" data-account-current-password :disabled="pending || saved" @update:model-value="delete errors.current"><template #prefix><LockKeyholeIcon :size="18" /></template></AppInput>
      </AppField>
      <AppField floating :label="t('shell.credentials.newPassword')" :error="errors.next" :hint="t('shell.credentials.passwordHint')" required>
        <AppInput v-model="newSecret" type="password" autocomplete="new-password" :disabled="pending || saved" @update:model-value="delete errors.next" />
      </AppField>
      <AppField floating :label="t('shell.credentials.confirmPassword')" :error="errors.confirm" required>
        <AppInput v-model="confirmSecret" type="password" autocomplete="new-password" :disabled="pending || saved" @update:model-value="delete errors.confirm" />
      </AppField>
      <AppButton variant="ghost" class="account-credentials__disclosure" :aria-expanded="showUsername" :aria-controls="usernameId" :disabled="pending || saved" @click="showUsername = !showUsername">
        <UserRoundPenIcon :size="18" />{{ t('shell.credentials.changeUsername') }}
        <motion.span class="account-credentials__arrow" :animate="{ rotate: showUsername ? 180 : 0 }" :transition="overlayMotion.transition"><ChevronDownIcon :size="16" /></motion.span>
      </AppButton>
      <AnimatePresence :initial="false">
        <motion.div v-if="showUsername" :id="usernameId" key="username" class="account-credentials__username" :initial="{ opacity: 0 }" :animate="{ opacity: 1 }" :exit="{ opacity: 0 }" :transition="overlayMotion.transition">
          <AppField floating :label="t('shell.credentials.newUsername')" :hint="t('shell.credentials.usernameHint')" :error="errors.identifier">
            <AppInput v-model="newIdentifier" autocomplete="username" :disabled="pending || saved" @update:model-value="delete errors.identifier" />
          </AppField>
        </motion.div>
      </AnimatePresence>
      <p v-if="feedback" class="account-credentials__error" role="alert">{{ feedback }}</p>
    </form>
    <template #footer>
      <div class="account-credentials__actions">
        <AppButton :disabled="pending || saved" @click="$emit('close')">{{ t('shell.cancel') }}</AppButton>
        <AppButton type="submit" :form="formId" variant="default" :loading="pending" :disabled="saved">{{ t('shell.credentials.save') }}</AppButton>
      </div>
    </template>
  </AppDialog>
</template>

<style scoped>
.account-credentials .app-field { margin-bottom: 20px; }
.account-credentials__disclosure { width: 100%; justify-content: flex-start; gap: 10px; padding-inline: 0; color: var(--muted); }
.account-credentials__disclosure:hover { background: transparent; color: var(--text); }
.account-credentials__arrow { display: inline-flex; margin-left: auto; }
.account-credentials__username { padding-top: 16px; }
.account-credentials__username .app-field { margin-bottom: 0; }
.account-credentials__error { margin: 16px 0 0; font-size: 13px; line-height: 1.6; color: var(--text-danger); }
.account-credentials__actions { display: flex; justify-content: flex-end; gap: 10px; }
</style>
