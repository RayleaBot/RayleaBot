<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import AppPage from '@/components/page/AppPage.vue'
import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import ConfigFieldRow from '@/components/config/ConfigFieldRow.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import { cloneConfig, getConfigSections, getValueByPath, setValueByPath, type ConfigFieldDefinition } from '@/lib/config-form'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

const configStore = useConfigStore()
const { document: configDocument, error, loading, redactedFields, restartRequired, saving } = storeToRefs(configStore)

const draft = ref<ConfigDocument | null>(null)
const configSections = computed(() => getConfigSections())
const activeSectionKey = ref<string>('server')
const sectionRefs = ref<HTMLElement[]>([])

watch(configDocument, (value) => {
  draft.value = value ? cloneConfig(value) : null
}, { immediate: true })

watch(configSections, (sections) => {
  if (!sections.some((section) => section.key === activeSectionKey.value)) {
    activeSectionKey.value = sections[0]?.key ?? 'server'
  }
}, { immediate: true })

const isDirty = computed(() => {
  if (!draft.value || !configDocument.value) {
    return false
  }
  return JSON.stringify(draft.value) !== JSON.stringify(configDocument.value)
})

const saveLabel = computed(() => (isDirty.value ? t('config.save') : t('config.saveIdle')))
const feedbackToast = computed(() => {
  if (error.value) {
    return {
      key: `config-error:${error.value}`,
      level: 'error' as const,
      message: error.value,
    }
  }

  if (redactedFields.value.length > 0) {
    return {
      key: `config-redacted:${redactedFields.value.join('|')}`,
      level: 'info' as const,
      message: `${t('config.redactedTitle')}：${redactedFields.value.join(', ')}`,
    }
  }

  return null
})

const segmentedOptions = computed(() =>
  configSections.value.map((section) => ({
    label: section.title,
    value: section.key,
  })),
)

async function loadConfig() {
  try {
    await configStore.fetchConfig()
  } catch {
    // store error state drives the page
  }
}

useToastFeedback(feedbackToast)

onMounted(() => {
  void loadConfig()
  nextTick(() => setupObserver())
})

let observer: IntersectionObserver | null = null

function teardownObserver() {
  if (observer) {
    observer.disconnect()
    observer = null
  }
}

function setupObserver() {
  teardownObserver()
  if (typeof IntersectionObserver === 'undefined') {
    return
  }

  const elements = sectionRefs.value.filter((el): el is HTMLElement => el instanceof HTMLElement)
  if (elements.length === 0) {
    return
  }

  const root = window.document.getElementById('app-main')

  observer = new IntersectionObserver(
    (entries) => {
      const visible = entries
        .filter((entry) => entry.isIntersecting)
        .sort((a, b) => (a.target as HTMLElement).offsetTop - (b.target as HTMLElement).offsetTop)
      if (visible.length === 0) {
        return
      }
      const key = (visible[0].target as HTMLElement).dataset.sectionKey
      if (key) {
        activeSectionKey.value = key
      }
    },
    {
      root,
      rootMargin: '-25% 0px -65% 0px',
      threshold: 0,
    },
  )

  elements.forEach((el) => observer?.observe(el))
}

watch(configSections, () => {
  nextTick(() => setupObserver())
})

watch(draft, (value, previous) => {
  if (!previous && value) {
    nextTick(() => setupObserver())
  }
})

onBeforeUnmount(() => {
  teardownObserver()
})

function scrollToSection(key: string) {
  const el = window.document.getElementById(`config-section-${key}`)
  if (!el) {
    return
  }
  const prefersReduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  el.scrollIntoView({ behavior: prefersReduced ? 'auto' : 'smooth', block: 'start' })
  activeSectionKey.value = key
}

function onSegmentedChange(value: string | number) {
  scrollToSection(String(value))
}

function readField(path: string, type: ConfigFieldDefinition['type']) {
  if (!draft.value) {
    if (type === 'boolean') {
      return false
    }
    return type === 'number' ? null : ''
  }
  return getValueByPath(draft.value as unknown as Record<string, unknown>, path)
}

function writeField(path: string, value: unknown) {
  if (!draft.value) {
    return
  }
  setValueByPath(draft.value as unknown as Record<string, unknown>, path, value)
}

async function save() {
  if (!draft.value) {
    return
  }
  const response = await configStore.saveConfig(draft.value)
  notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
}
</script>

<template>
  <AppPage :title="t('config.title')">
    <RetryPanel
      v-if="error && !draft"
      :title="t('config.title')"
      :description="error"
      :loading="loading"
      @retry="loadConfig"
    />

    <div v-else-if="loading && !draft" class="config-skeleton">
      <AppSkeletonCard show-header :rows="8" />
      <AppSkeletonCard show-header :rows="6" />
    </div>

    <div v-else-if="draft" class="config-page">
      <div class="config-toolbar" role="region" :aria-label="t('config.title')">
        <div class="config-toolbar__status">
          <span
            class="config-toolbar__dirty"
            :class="{ 'is-active': isDirty }"
            :aria-label="isDirty ? t('config.dirtyDotLabel') : undefined"
          />
          <span class="config-toolbar__status-text">{{ saveLabel }}</span>
          <a-tag
            v-if="restartRequired !== null"
            :color="restartRequired ? 'warning' : 'success'"
            class="config-toolbar__tag"
          >
            {{ restartRequired ? t('config.restartNeeded') : t('config.hotApplied') }}
          </a-tag>
        </div>
        <div class="config-toolbar__actions">
          <a-button :loading="loading" :aria-label="t('dashboard.refresh')" @click="loadConfig">
            {{ t('dashboard.refresh') }}
          </a-button>
          <a-button
            type="primary"
            :disabled="!isDirty || saving"
            :loading="saving"
            :aria-label="t('config.save')"
            @click="save"
          >
            {{ t('config.save') }}
          </a-button>
        </div>
      </div>

      <div class="config-toc-inline" :aria-label="t('config.tocLabel')">
        <a-segmented
          :value="activeSectionKey"
          :options="segmentedOptions"
          @change="onSegmentedChange"
        />
      </div>

      <div class="config-grid">
        <div class="config-stack">
          <section
            v-for="section in configSections"
            :key="section.key"
            :id="`config-section-${section.key}`"
            :data-section-key="section.key"
            ref="sectionRefs"
            class="config-section"
          >
            <header class="config-section__head">
              <h2 class="config-section__title">{{ section.title }}</h2>
              <p v-if="section.description" class="config-section__desc">{{ section.description }}</p>
            </header>
            <div class="config-section__fields">
              <ConfigFieldRow
                v-for="field in section.fields"
                :key="field.path"
                :field="field"
                :value="readField(field.path, field.type)"
                @update:value="(value) => writeField(field.path, value)"
              />
            </div>
          </section>
        </div>

        <aside class="config-toc" :aria-label="t('config.tocLabel')">
          <div class="config-toc__sticky">
            <p class="config-toc__heading">{{ t('config.tocLabel') }}</p>
            <nav class="config-toc__list">
              <a
                v-for="section in configSections"
                :key="section.key"
                :href="`#config-section-${section.key}`"
                class="config-toc__item"
                :class="{ 'is-active': activeSectionKey === section.key }"
                :aria-current="activeSectionKey === section.key ? 'true' : undefined"
                @click.prevent="scrollToSection(section.key)"
              >
                <span class="config-toc__label">{{ section.title }}</span>
                <span class="config-toc__count">{{ section.fields.length }}</span>
              </a>
            </nav>
          </div>
        </aside>
      </div>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.config-skeleton {
  display: grid;
  gap: var(--space-md);
}

.config-page {
  container-type: inline-size;
  container-name: configpage;
  display: grid;
  gap: var(--space-md);
}

.config-toolbar {
  position: sticky;
  top: 0;
  z-index: 5;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  padding: 10px var(--space-md);
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  backdrop-filter: blur(8px);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xs);
}

.config-toolbar__status {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 0.85rem;
  color: var(--muted);
  min-width: 0;
}

.config-toolbar__dirty {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: transparent;
  border: 1px solid var(--border-strong);
  transition: background 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
  flex-shrink: 0;
}

.config-toolbar__dirty.is-active {
  background: var(--accent);
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.config-toolbar__status-text {
  font-weight: 500;
}

.config-toolbar__tag {
  margin-inline-start: 4px;
}

.config-toolbar__actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.config-toc-inline {
  display: none;
}

.config-toc-inline :deep(.ant-segmented) {
  width: 100%;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  background: var(--surface-soft);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 4px;
}

.config-toc-inline :deep(.ant-segmented-item) {
  scroll-snap-align: start;
}

.config-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 220px;
  gap: var(--space-xl);
  align-items: start;
}

.config-stack {
  display: grid;
  gap: var(--space-2xl);
  min-width: 0;
}

.config-section {
  scroll-margin-top: 80px;
  display: grid;
  gap: var(--space-lg);
  padding-block-end: var(--space-xl);
  border-bottom: 1px solid var(--border);
}

.config-section:last-child {
  border-bottom: none;
  padding-block-end: 0;
}

.config-section__head {
  display: grid;
  gap: 4px;
}

.config-section__title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text);
  letter-spacing: -0.01em;
}

.config-section__desc {
  margin: 0;
  color: var(--muted);
  font-size: 0.85rem;
  line-height: 1.55;
  max-width: 68ch;
}

.config-section__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: clamp(var(--space-md), 1.6vw, var(--space-xl)) clamp(var(--space-lg), 2vw, var(--space-2xl));
}

.config-toc {
  align-self: start;
  min-width: 0;
}

.config-toc__sticky {
  position: sticky;
  top: 72px;
  display: grid;
  gap: 8px;
  padding: 6px 0;
}

.config-toc__heading {
  margin: 0 0 4px 12px;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
}

.config-toc__list {
  display: grid;
  gap: 2px;
}

.config-toc__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 12px;
  border-left: 2px solid transparent;
  color: var(--muted);
  font-size: 0.84rem;
  text-decoration: none;
  line-height: 1.5;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.config-toc__item:hover {
  color: var(--text);
}

.config-toc__item.is-active {
  color: var(--text-accent);
  border-left-color: var(--accent);
  font-weight: 600;
}

.config-toc__item:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
  border-radius: 2px;
}

.config-toc__count {
  font-size: 0.72rem;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  font-weight: 500;
}

.config-toc__item.is-active .config-toc__count {
  color: var(--text-accent);
}

@container configpage (max-width: 960px) {
  .config-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .config-toc {
    display: none;
  }

  .config-toc-inline {
    display: block;
  }
}

@container configpage (max-width: 640px) {
  .config-section__fields {
    grid-template-columns: minmax(0, 1fr);
  }

  .config-toolbar {
    flex-wrap: wrap;
    gap: 10px;
  }

  .config-toolbar__actions {
    flex: 1 1 auto;
    justify-content: flex-end;
  }
}
</style>
