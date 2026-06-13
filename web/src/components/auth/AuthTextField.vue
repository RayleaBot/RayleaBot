<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { EyeInvisibleOutlined, EyeOutlined } from '@ant-design/icons-vue'

import { t } from '@/i18n'

const props = withDefaults(defineProps<{
  label: string
  name: string
  type?: 'text' | 'password'
  autocomplete?: string
  error?: string | null
}>(), {
  type: 'text',
  autocomplete: undefined,
  error: null,
})

const model = defineModel<string>({ default: '' })

const inputRef = ref<HTMLInputElement | null>(null)
const secretVisible = ref(false)
const inputId = useId()
const errorId = useId()

const effectiveType = computed(() => {
  if (props.type !== 'password') {
    return props.type
  }
  return secretVisible.value ? 'text' : 'password'
})

function focus() {
  inputRef.value?.focus()
}

defineExpose({ focus })
</script>

<template>
  <div class="auth-field" :class="{ 'has-error': !!error }">
    <div class="auth-field__control">
      <input
        :id="inputId"
        ref="inputRef"
        v-model="model"
        class="auth-field__input"
        :type="effectiveType"
        :name="name"
        placeholder=" "
        :autocomplete="autocomplete"
        aria-required="true"
        :aria-invalid="error ? 'true' : undefined"
        :aria-describedby="error ? errorId : undefined"
      >
      <label class="auth-field__label" :for="inputId">{{ label }}</label>
      <button
        v-if="type === 'password'"
        type="button"
        class="auth-field__eye"
        :aria-label="secretVisible ? t('auth.hideSecret') : t('auth.showSecret')"
        :aria-pressed="secretVisible"
        @click="secretVisible = !secretVisible"
      >
        <EyeOutlined v-if="secretVisible" />
        <EyeInvisibleOutlined v-else />
      </button>
    </div>
    <p
      v-if="error"
      :id="errorId"
      class="auth-field__error"
      role="alert"
    >
      {{ error }}
    </p>
  </div>
</template>

<style scoped lang="scss">
.auth-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.auth-field__control {
  position: relative;
  display: flex;
  align-items: center;
  border: 1px solid color-mix(in srgb, var(--text) 14%, transparent);
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface) 46%, transparent);
  transition: border-color 0.25s ease, box-shadow 0.25s ease, background-color 0.25s ease;

  &:hover {
    border-color: color-mix(in srgb, var(--auth-accent) 45%, var(--text) 12%);
  }

  &:focus-within {
    border-color: var(--auth-accent);
    background: color-mix(in srgb, var(--surface) 62%, transparent);
    box-shadow: 0 0 0 4px color-mix(in srgb, var(--auth-accent) 18%, transparent);
  }
}

.auth-field__input {
  flex: 1;
  min-width: 0;
  height: 54px;
  padding: 22px 16px 6px;
  border: 0;
  background: transparent;
  color: var(--text);
  font-family: inherit;
  font-size: 15px;
  outline: none;

  &::placeholder {
    color: transparent;
  }

  &:-webkit-autofill {
    -webkit-text-fill-color: var(--text);
    transition: background-color 999999s ease;
  }
}

.auth-field__label {
  position: absolute;
  top: 18px;
  left: 16px;
  color: var(--muted);
  font-size: 15px;
  line-height: 18px;
  pointer-events: none;
  transform-origin: left top;
  transition: transform 0.18s ease, color 0.18s ease;
}

.auth-field__input:focus + .auth-field__label,
.auth-field__input:not(:placeholder-shown) + .auth-field__label {
  transform: translateY(-11px) scale(0.76);
  color: color-mix(in srgb, var(--auth-accent-deep) 70%, var(--muted));
}

.auth-field__eye {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  margin-right: 7px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--muted);
  font-size: 16px;
  cursor: pointer;
  transition: color 0.2s ease, background-color 0.2s ease;

  &:hover {
    background: color-mix(in srgb, var(--text) 7%, transparent);
    color: var(--text);
  }

  &:focus-visible {
    outline: 2px solid var(--auth-accent);
    outline-offset: 1px;
  }
}

.auth-field__error {
  margin: 0;
  padding-left: 4px;
  color: var(--danger);
  font-size: 12.5px;
  line-height: 1.4;
}

.auth-field.has-error {
  .auth-field__control {
    border-color: color-mix(in srgb, var(--danger) 65%, transparent);

    &:focus-within {
      border-color: var(--danger);
      box-shadow: 0 0 0 4px color-mix(in srgb, var(--danger) 14%, transparent);
    }
  }

  .auth-field__label {
    color: var(--danger);
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-field__control,
  .auth-field__label,
  .auth-field__eye {
    transition: none;
  }
}
</style>
