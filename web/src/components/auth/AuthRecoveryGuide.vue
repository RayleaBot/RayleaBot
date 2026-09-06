<script setup lang="ts">
import { ref } from 'vue'
import { ArrowLeftOutlined, DesktopOutlined, CodeOutlined } from '@ant-design/icons-vue'

import AuthPanel from '@/components/auth/AuthPanel.vue'
import { t } from '@/i18n'

defineEmits<{ back: [] }>()

const panel = ref<InstanceType<typeof AuthPanel> | null>(null)
defineExpose({ focus: () => panel.value?.focus() })
</script>

<template>
  <AuthPanel
    ref="panel"
    heading-id="auth-recovery-title"
    :title="t('auth.recovery.title')"
    :subtitle="t('auth.recovery.body')"
  >
    <div class="auth-recovery">
      <section class="auth-recovery__method" aria-labelledby="auth-recovery-launcher">
        <h2 id="auth-recovery-launcher"><DesktopOutlined aria-hidden="true" />{{ t('auth.recovery.launcherTitle') }}</h2>
        <p>{{ t('auth.recovery.launcherBody') }}</p>
      </section>
      <section class="auth-recovery__method" aria-labelledby="auth-recovery-cli">
        <h2 id="auth-recovery-cli"><CodeOutlined aria-hidden="true" />{{ t('auth.recovery.cliTitle') }}</h2>
        <p>{{ t('auth.recovery.cliBody') }}</p>
        <div class="auth-recovery__commands">
          <span class="auth-recovery__platform">{{ t('auth.recovery.cliWindows') }}</span>
          <code class="auth-recovery__command">.\raylea-server.exe reset-admin</code>
          <span class="auth-recovery__platform">{{ t('auth.recovery.cliUnix') }}</span>
          <code class="auth-recovery__command">./raylea-server reset-admin</code>
        </div>
        <p class="auth-recovery__hint">{{ t('auth.recovery.configHint') }}</p>
      </section>
      <p class="auth-recovery__next">{{ t('auth.recovery.next') }}</p>
      <p class="auth-recovery__note">{{ t('auth.recovery.retained') }}</p>
    </div>
    <template #footer>
      <button class="auth-panel__text-action" type="button" @click="$emit('back')">
        <ArrowLeftOutlined aria-hidden="true" />{{ t('auth.recovery.back') }}
      </button>
    </template>
  </AuthPanel>
</template>

<style scoped lang="scss">
.auth-recovery { margin-top: 28px; }
.auth-recovery__method + .auth-recovery__method { margin-top: 24px; }
.auth-recovery__method h2 {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 0 0 8px;
  color: var(--auth-text);
  font-size: 14px;
  font-weight: 600;
  line-height: 1.5;
  .anticon { color: var(--auth-brand-foreground); font-size: 16px; }
}
.auth-recovery p { margin: 0; color: var(--auth-text-muted); font-size: 13px; line-height: 1.8; }
.auth-recovery__commands {
  margin-top: 12px;
  padding: 12px 14px;
  color: var(--auth-text);
  border: 1px solid var(--auth-glass-divider);
  border-radius: 12px;
  background: var(--auth-glass-control);
}
.auth-recovery__platform { display: block; margin-bottom: 4px; color: var(--auth-text-muted); font-size: 12px; }
.auth-recovery__command + .auth-recovery__platform { margin-top: 12px; }
.auth-recovery__command {
  display: block;
  font-family: var(--font-mono);
  font-size: 12px;
  overflow-wrap: anywhere;
  user-select: all;
}
.auth-recovery .auth-recovery__hint { margin-top: 8px; font-size: 12px; }
.auth-recovery .auth-recovery__next { margin-top: 24px; color: var(--auth-text); }
.auth-recovery .auth-recovery__note { margin-top: 8px; font-size: 12px; }
@media (forced-colors: active) {
  .auth-recovery__commands { border-color: CanvasText; }
}
</style>
