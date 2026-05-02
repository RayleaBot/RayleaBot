<script setup lang="ts">
import { computed } from 'vue'
import { ArrowLeftOutlined, ReloadOutlined } from '@ant-design/icons-vue'

import Icon403 from '@/components/fallback/icons/Icon403.vue'
import Icon404 from '@/components/fallback/icons/Icon404.vue'
import Icon500 from '@/components/fallback/icons/Icon500.vue'
import IconOffline from '@/components/fallback/icons/IconOffline.vue'
import { t } from '@/i18n'
import type { ExceptionStatus } from '@/lib/exception-status'

const props = withDefaults(defineProps<{
  description?: string
  homeLabel?: string
  retryLabel?: string
  retryLoading?: boolean
  showHome?: boolean
  showRetry?: boolean
  status: ExceptionStatus
  title?: string
}>(), {
  description: '',
  homeLabel: '',
  retryLabel: '',
  retryLoading: false,
  showHome: true,
  showRetry: true,
  title: '',
})

const emit = defineEmits<{
  home: []
  retry: []
}>()

const fallbackIcon = computed(() => {
  switch (props.status) {
    case '403':
      return Icon403
    case '404':
      return Icon404
    case '500':
      return Icon500
    case 'offline':
      return IconOffline
    default:
      return null
  }
})

const titleText = computed(() => props.title || t(`fallback.status.${props.status}.title`))
const descriptionText = computed(() => props.description || t(`fallback.status.${props.status}.description`))
const homeButtonLabel = computed(() => props.homeLabel || t('fallback.actions.backHome'))
const retryButtonLabel = computed(() => props.retryLabel || (
  props.status === 'offline' ? t('fallback.actions.recheck') : t('fallback.actions.retry')
))
</script>

<template>
  <section class="vben-fallback" role="alert" :data-status="status" data-testid="vben-fallback">
    <component :is="fallbackIcon" v-if="fallbackIcon" class="vben-fallback__visual" aria-hidden="true" />

    <div class="vben-fallback__content">
      <h1>{{ titleText }}</h1>
      <p>{{ descriptionText }}</p>

      <div class="vben-fallback__actions">
        <a-button v-if="showHome" size="large" @click="emit('home')">
          <template #icon><ArrowLeftOutlined /></template>
          {{ homeButtonLabel }}
        </a-button>
        <a-button v-if="showRetry" type="primary" size="large" :loading="retryLoading" @click="emit('retry')">
          <template #icon><ReloadOutlined /></template>
          {{ retryButtonLabel }}
        </a-button>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss">
.vben-fallback {
  --primary: 220 86% 48%;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 28px;
  min-height: min(720px, calc(100vh - 150px));
  width: 100%;
  padding: 48px 24px;
  color: var(--text);
  text-align: center;
}

[data-theme='dark'] .vben-fallback {
  --primary: 218 100% 65%;
}

.vben-fallback__visual {
  display: block;
  width: min(360px, 58vw);
  max-height: 38vh;
  color: var(--accent);
}

.vben-fallback__content {
  display: grid;
  justify-items: center;
  gap: 14px;
  max-width: 560px;
}

.vben-fallback__content h1 {
  margin: 0;
  font-size: clamp(1.7rem, 3vw, 2.5rem);
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
}

.vben-fallback__content p {
  margin: 0;
  color: var(--muted);
  font-size: clamp(0.95rem, 1.2vw, 1.1rem);
  line-height: 1.65;
}

.vben-fallback__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  margin-top: 6px;
}

.vben-fallback__actions :deep(.ant-btn) {
  min-width: 112px;
}

@media (max-width: 720px) {
  .vben-fallback {
    min-height: calc(100vh - 132px);
    padding: 36px 18px;
  }

  .vben-fallback__visual {
    width: min(300px, 76vw);
  }
}
</style>
