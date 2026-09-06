<script setup lang="ts">
import { h, nextTick, reactive, ref, watch } from 'vue'
import { ArrowRightOutlined, EyeInvisibleOutlined, EyeOutlined } from '@ant-design/icons-vue'

import AuthPanel from '@/components/auth/AuthPanel.vue'
import { t } from '@/i18n'

type AuthFormFeedback = {
  level: 'error' | 'warning'
  message: string
}

type FocusableInput = {
  focus: () => void
}

const props = withDefaults(defineProps<{
  feedback?: AuthFormFeedback | null
  pending: boolean
  secretAutocomplete: 'current-password' | 'new-password'
  submitLabel: string
  subtitle: string
  title: string
}>(), {
  feedback: null,
})

const emit = defineEmits<{
  change: []
  submit: [payload: { identifier: string, secret: string }]
}>()

const credentials = reactive({
  identifier: 'admin',
  secret: '',
})
const errors = reactive<{ identifier: string | null, secret: string | null }>({
  identifier: null,
  secret: null,
})
const identifierField = ref<FocusableInput | null>(null)
const secretField = ref<FocusableInput | null>(null)

watch(() => credentials.identifier, (value) => {
  if (value.trim()) {
    errors.identifier = null
  }
  emit('change')
})

watch(() => credentials.secret, (value) => {
  if (value) {
    errors.secret = null
  }
  emit('change')
})

function renderPasswordIcon(visible: boolean) {
  return h(
    'button',
    {
      'aria-label': visible ? t('auth.hideSecret') : t('auth.showSecret'),
      'aria-pressed': String(visible),
      type: 'button',
    },
    [h(visible ? EyeOutlined : EyeInvisibleOutlined)],
  )
}

async function handleSubmit() {
  if (props.pending) {
    return
  }

  errors.identifier = credentials.identifier.trim() ? null : t('auth.validation.identifierRequired')
  errors.secret = credentials.secret ? null : t('auth.validation.secretRequired')

  if (errors.identifier || errors.secret) {
    await nextTick()
    if (errors.identifier) {
      identifierField.value?.focus()
    } else {
      secretField.value?.focus()
    }
    return
  }

  emit('submit', {
    identifier: credentials.identifier,
    secret: credentials.secret,
  })
}
</script>

<template>
  <AuthPanel :title="title" :subtitle="subtitle">
    <a-form
      class="auth-form"
      layout="vertical"
      :model="credentials"
      novalidate
      @submit="handleSubmit"
    >
      <a-form-item
        html-for="auth-identifier"
        :label="t('auth.identifier')"
        name="identifier"
        :validate-status="errors.identifier ? 'error' : undefined"
      >
        <template v-if="errors.identifier" #help>
          <span id="auth-identifier-error" role="alert">{{ errors.identifier }}</span>
        </template>
        <a-input
          id="auth-identifier"
          ref="identifierField"
          class="auth-form__control auth-form__control--identifier"
          v-model:value="credentials.identifier"
          autocomplete="username"
          :disabled="pending"
          name="identifier"
          aria-required="true"
          :aria-describedby="errors.identifier ? 'auth-identifier-error' : undefined"
          :aria-invalid="errors.identifier ? 'true' : undefined"
        />
      </a-form-item>

      <a-form-item
        html-for="auth-secret"
        :label="t('auth.secret')"
        name="secret"
        :validate-status="errors.secret ? 'error' : undefined"
      >
        <template v-if="errors.secret" #help>
          <span id="auth-secret-error" role="alert">{{ errors.secret }}</span>
        </template>
        <a-input-password
          id="auth-secret"
          ref="secretField"
          class="auth-form__control auth-form__control--secret"
          v-model:value="credentials.secret"
          :autocomplete="secretAutocomplete"
          :disabled="pending"
          :icon-render="renderPasswordIcon"
          :placeholder="t(secretAutocomplete === 'new-password' ? 'auth.newSecretPlaceholder' : 'auth.secretPlaceholder')"
          name="secret"
          aria-required="true"
          :aria-describedby="errors.secret ? 'auth-secret-error' : undefined"
          :aria-invalid="errors.secret ? 'true' : undefined"
        />
      </a-form-item>

      <a-alert
        v-if="feedback"
        class="auth-form__feedback"
        :message="feedback.message"
        :role="feedback.level === 'error' ? 'alert' : 'status'"
        show-icon
        :type="feedback.level"
      />

      <a-button
        class="auth-form__submit"
        block
        html-type="submit"
        :loading="pending"
        type="primary"
        :aria-busy="pending || undefined"
      >
        <span>{{ submitLabel }}</span>
        <ArrowRightOutlined v-if="!pending" aria-hidden="true" />
      </a-button>
    </a-form>
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </AuthPanel>
</template>

<style scoped lang="scss">
.auth-form {
  display: grid;
  gap: 22px;
  margin-top: 32px;
}

.auth-form :deep(.ant-form-item) {
  margin-bottom: 0;
}

.auth-form :deep(.ant-form-item-label) {
  padding-bottom: 6px;
}

.auth-form :deep(.ant-form-item-label > label) {
  height: auto;
  color: var(--auth-text);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
}

.auth-form :deep(.auth-form__control--identifier.ant-input),
.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper) {
  box-sizing: border-box;
  height: 50px;
  min-height: 50px;
  color: var(--auth-text);
  font-size: 16px;
  border: 1px solid var(--auth-border-control);
  border-radius: 16px;
  outline: none;
  background: var(--auth-glass-control, var(--auth-control));
  box-shadow: none;
  transition:
    color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    background-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    border-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    box-shadow 180ms cubic-bezier(0.16, 1, 0.3, 1);
}

.auth-form :deep(.auth-form__control--identifier.ant-input) {
  padding-inline: 14px 48px;
}

.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper) {
  padding-inline: 14px 8px;
}

.auth-form :deep(.auth-form__control--identifier.ant-input:hover),
.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper:hover) {
  border-color: var(--auth-brand-stroke, var(--auth-brand-foreground));
  background: var(--auth-glass-control-hover, var(--auth-control-hover));
}

.auth-form :deep(.auth-form__control--identifier.ant-input:focus),
.auth-form :deep(.auth-form__control--identifier.ant-input:focus-visible),
.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper-focused),
.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper:focus-within) {
  border-width: 1px;
  border-color: var(--auth-brand-stroke, var(--auth-brand-foreground));
  outline: none;
  background: var(--auth-glass-control-hover, var(--auth-control-hover));
  box-shadow:
    0 0 0 3px color-mix(in srgb, var(--auth-brand-foreground) 22%, transparent),
    0 2px 8px color-mix(in srgb, var(--auth-brand-foreground) 8%, transparent);
}

.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper > .ant-input) {
  min-height: 32px;
  padding-right: 40px;
  outline: none;
  background: transparent;
  box-shadow: none;
}

.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper > .ant-input:focus),
.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper > .ant-input:focus-visible) {
  outline: none;
  box-shadow: none;
}

.auth-form :deep(.auth-form__control--identifier.ant-input-status-error),
.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper-status-error) {
  border-width: 1px;
  border-color: var(--auth-danger);
}

.auth-form :deep(.auth-form__control--identifier.ant-input:disabled),
.auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper-disabled) {
  color: var(--auth-text-muted);
  border-width: 1px;
  border-color: var(--auth-border);
  background: color-mix(in srgb, var(--auth-control) 72%, var(--auth-canvas));
}

.auth-form :deep(input:-webkit-autofill),
.auth-form :deep(input:-webkit-autofill:hover),
.auth-form :deep(input:-webkit-autofill:focus) {
  box-shadow: 0 0 0 1000px var(--auth-control) inset;
  -webkit-text-fill-color: var(--auth-text);
  caret-color: var(--auth-text);
}

.auth-form :deep(.ant-form-item-explain-error) {
  padding-top: 4px;
  font-size: 13px;
  line-height: 1.4;
}

.auth-form :deep(.ant-input-password-icon) {
  display: grid;
  width: 32px;
  height: 32px;
  padding: 0;
  place-items: center;
  color: var(--auth-text-muted);
  border: 0;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  transition: color 160ms cubic-bezier(0.16, 1, 0.3, 1), background-color 160ms cubic-bezier(0.16, 1, 0.3, 1);
}

.auth-form :deep(.ant-input-password-icon:hover),
.auth-form :deep(.ant-input-password-icon:focus-visible) {
  color: var(--auth-brand-foreground);
  background: var(--auth-brand-soft);
  outline: 2px solid var(--auth-focus);
  outline-offset: 2px;
}

.auth-form__feedback {
  border-radius: 16px;
}

.auth-form__submit.ant-btn {
  height: 50px;
  min-height: 50px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 6px;
  color: var(--auth-on-brand);
  border-color: var(--auth-brand-fill);
  border-radius: 16px;
  background: var(--auth-brand-fill);
  box-shadow: 0 6px 16px -6px color-mix(in srgb, var(--auth-brand-fill) 50%, transparent), inset 0 1px 0 color-mix(in srgb, var(--auth-glass-edge) 35%, transparent);
  font-weight: 600;
  transition:
    color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    background-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    border-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    box-shadow 160ms cubic-bezier(0.16, 1, 0.3, 1);
}

.auth-form__submit.ant-btn:not(:disabled):hover {
  border-color: var(--auth-brand-fill-hover);
  background: var(--auth-brand-fill-hover);
}

.auth-form__submit.ant-btn:not(:disabled):focus-visible {
  border-color: var(--auth-brand-fill-hover);
  background: var(--auth-brand-fill-hover);
  outline: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--auth-brand-foreground) 28%, transparent);
}

.auth-form__submit.ant-btn:not(:disabled):active {
  border-color: var(--auth-brand-fill-pressed);
  background: var(--auth-brand-fill-pressed);
  box-shadow: none;
}

@media (max-width: 600px) {
  .auth-form {
    gap: 16px;
  }

  .auth-form :deep(.ant-input-password-icon) {
    width: 44px;
    height: 44px;
  }
}

@media (pointer: coarse) {
  .auth-form :deep(.ant-input-password-icon) { width: 44px; height: 44px; }
}

@media (prefers-reduced-motion: reduce) {
  .auth-form :deep(.auth-form__control--identifier.ant-input),
  .auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper),
  .auth-form :deep(.ant-input-password-icon),
  .auth-form__submit.ant-btn {
    transition: none;
  }
}

@media (forced-colors: active) {
  .auth-form__submit.ant-btn:not(:disabled):focus-visible {
    outline: 2px solid Highlight;
    outline-offset: 2px;
    box-shadow: none;
  }

  .auth-form :deep(.auth-form__control--identifier.ant-input),
  .auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper) {
    border-color: CanvasText;
  }

  .auth-form :deep(.auth-form__control--identifier.ant-input:focus),
  .auth-form :deep(.auth-form__control--identifier.ant-input:focus-visible),
  .auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper-focused),
  .auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper:focus-within) {
    outline: 2px solid Highlight;
    outline-offset: 2px;
    box-shadow: none;
  }

  .auth-form :deep(.auth-form__control--identifier.ant-input-status-error),
  .auth-form :deep(.auth-form__control--secret.ant-input-affix-wrapper-status-error) {
    border-color: Mark;
  }
}
</style>
