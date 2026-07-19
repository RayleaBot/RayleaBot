<script setup lang="ts">
import { h, nextTick, reactive, ref, watch } from 'vue'
import { EyeInvisibleOutlined, EyeOutlined } from '@ant-design/icons-vue'

import RayleaMark from '@/components/brand/RayleaMark.vue'
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
  <section class="auth-panel" aria-labelledby="auth-panel-title">
    <header class="auth-panel__header">
      <div class="auth-panel__brand-row">
        <RayleaMark variant="neutral" />
        <p class="auth-panel__brand">{{ t('app.brand') }} {{ t('auth.surface') }}</p>
      </div>
      <h1 id="auth-panel-title" class="auth-panel__title">{{ title }}</h1>
      <p class="auth-panel__subtitle">{{ subtitle }}</p>
    </header>

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
          v-model:value="credentials.secret"
          :autocomplete="secretAutocomplete"
          :disabled="pending"
          :icon-render="renderPasswordIcon"
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
        {{ submitLabel }}
      </a-button>
    </a-form>
  </section>
</template>

<style scoped lang="scss">
.auth-panel {
  width: 100%;
  padding: 40px;
  color: var(--auth-text);
}

.auth-panel__header {
  min-height: 132px;
  padding: 2px 56px 28px 0;
  border-bottom: 1px solid var(--auth-border);
}

.auth-panel__brand {
  margin: 0;
  color: var(--auth-brand-foreground);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
}

.auth-panel__brand-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.auth-panel__title {
  margin: 0;
  color: var(--auth-text);
  font-size: 24px;
  font-weight: 700;
  line-height: 1.25;
  letter-spacing: -0.015em;
}

.auth-panel__subtitle {
  max-width: 34ch;
  margin: 10px 0 0;
  color: var(--auth-text-muted);
  font-size: 14px;
  line-height: 1.55;
}

.auth-form {
  display: grid;
  gap: 20px;
  margin-top: 28px;
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

.auth-form :deep(.ant-input),
.auth-form :deep(.ant-input-affix-wrapper) {
  border-color: var(--auth-border);
  border-radius: 10px;
  background: var(--auth-control);
  transition:
    color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    background-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    border-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    box-shadow 160ms cubic-bezier(0.16, 1, 0.3, 1);
}

.auth-form :deep(.ant-input:hover),
.auth-form :deep(.ant-input-affix-wrapper:hover) {
  border-color: var(--auth-brand-stroke, var(--auth-brand-foreground));
  background: var(--auth-control-hover);
}

.auth-form :deep(.ant-input:focus),
.auth-form :deep(.ant-input-affix-wrapper-focused) {
  border-color: var(--auth-brand-stroke, var(--auth-brand-foreground));
  background: var(--auth-control-hover);
}

.auth-form :deep(.ant-input-affix-wrapper:has(input:focus-visible)) {
  outline: 2px solid var(--auth-focus);
  outline-offset: 2px;
}

.auth-form :deep(.ant-input-affix-wrapper .ant-input) {
  min-height: auto;
  background: transparent;
}

.auth-form :deep(.ant-input:disabled),
.auth-form :deep(.ant-input-affix-wrapper-disabled) {
  color: var(--auth-text-muted);
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
  border-radius: 10px;
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
  border-radius: 10px;
}

.auth-form__submit.ant-btn {
  margin-top: 4px;
  color: var(--auth-on-brand);
  border-color: var(--auth-brand-fill);
  border-radius: 10px;
  background: var(--auth-brand-fill);
  box-shadow: var(--auth-primary-shadow);
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
  outline: 2px solid var(--auth-focus);
  outline-offset: 2px;
  box-shadow: none;
}

.auth-form__submit.ant-btn:not(:disabled):active {
  border-color: var(--auth-brand-fill-pressed);
  background: var(--auth-brand-fill-pressed);
  box-shadow: none;
}

@media (max-width: 600px) {
  .auth-panel {
    padding: 24px;
  }

  .auth-panel__header {
    min-height: 124px;
    padding: 0 48px 24px 0;
  }

  .auth-form {
    gap: 16px;
    margin-top: 24px;
  }

  .auth-form :deep(.ant-input),
  .auth-form :deep(.ant-input-affix-wrapper),
  .auth-form__submit.ant-btn {
    min-height: 44px;
  }

  .auth-form :deep(.ant-input-password-icon) {
    width: 36px;
    height: 36px;
  }
}

@media (pointer: coarse) {
  .auth-form :deep(.ant-input),
  .auth-form :deep(.ant-input-affix-wrapper),
  .auth-form__submit.ant-btn {
    min-height: 44px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-form :deep(.ant-input),
  .auth-form :deep(.ant-input-affix-wrapper),
  .auth-form :deep(.ant-input-password-icon),
  .auth-form__submit.ant-btn {
    transition: none;
  }
}
</style>
