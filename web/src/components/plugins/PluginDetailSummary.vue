<script setup lang="ts">
import { computed } from 'vue'
import { ChevronDownIcon } from '@lucide/vue'

import AppTag from '@/components/AppTag.vue'
import { t } from '@/i18n'
import { formatPluginVersion, getPluginTrustLabel } from '@/lib/display'
import { safeJsonStringify } from '@/lib/text-safety'
import type { PluginDetail } from '@/types/api'

const props = defineProps<{ plugin: PluginDetail | null }>()

const sections = computed(() => {
  const plugin = props.plugin
  return [
    {
      key: 'package',
      title: t('plugins.sections.packageInfo'),
      rows: [
        { key: 'author', label: t('plugins.fields.author'), value: textOrEmpty(plugin?.author) },
        { key: 'license', label: t('plugins.fields.license'), value: textOrEmpty(plugin?.license) },
        { key: 'core', label: t('plugins.fields.minCoreVersion'), value: formatPluginVersion(plugin?.min_core_version) },
      ],
    },
    {
      key: 'source',
      title: t('plugins.sections.sourceInfo'),
      rows: [
        { key: 'root', label: t('plugins.fields.sourceRoot'), value: textOrEmpty(plugin?.source?.root) },
        { key: 'ref', label: t('plugins.fields.sourceRef'), value: textOrEmpty(plugin?.source?.package_source_ref ?? plugin?.source?.package_source_type) },
        { key: 'trust', label: t('plugins.fields.trust'), value: getPluginTrustLabel(plugin?.trust?.level) },
      ],
    },
    {
      key: 'runtime',
      title: t('plugins.sections.runtimeConfig'),
      rows: [
        { key: 'concurrency', label: t('plugins.fields.concurrency'), value: plugin?.concurrency ?? t('display.empty') },
        { key: 'priority', label: t('plugins.fields.priority'), value: plugin?.priority ?? 0 },
        { key: 'block', label: t('plugins.fields.propagation'), value: t(plugin?.block ? 'plugins.propagation.stop' : 'plugins.propagation.continue') },
      ],
    },
  ]
})

function textOrEmpty(value?: string | null) {
  return value?.trim() || t('display.empty')
}

function hasItems(value?: readonly unknown[] | null) {
  return Array.isArray(value) && value.length > 0
}
</script>

<template>
  <section class="plugin-detail-summary-panel" :aria-label="t('plugins.sections.runtimeSummary')">
    <div class="plugin-detail-summary-stack">
      <section v-for="section in sections" :key="section.key" class="plugin-detail-summary-section">
        <h3>{{ section.title }}</h3>
        <dl class="plugin-detail-kv-list">
          <div v-for="item in section.rows" :key="item.key">
            <dt>{{ item.label }}</dt>
            <dd>{{ item.value }}</dd>
          </div>
        </dl>
        <div v-if="section.key === 'runtime'" class="metadata-section">
          <strong>{{ t('plugins.fields.events') }}</strong>
          <div v-if="hasItems(plugin?.events)" class="tag-list">
            <AppTag v-for="eventName in plugin?.events" :key="eventName">{{ eventName }}</AppTag>
          </div>
          <p v-else class="empty-val">{{ t('display.empty') }}</p>
        </div>
      </section>

      <details class="plugin-detail-disclosure">
        <summary>
          <span>{{ t('plugins.sections.details') }}</span>
          <AppTag size="small">{{ t('plugins.sections.metadata') }}</AppTag>
          <ChevronDownIcon class="plugin-detail-disclosure__chevron" :size="16" aria-hidden="true" />
        </summary>

        <div class="plugin-detail-detail-stack">
          <section class="metadata-section">
            <strong>{{ t('plugins.fields.description') }}</strong>
            <p class="meta-desc">{{ textOrEmpty(plugin?.description) }}</p>
          </section>

          <section class="metadata-section">
            <strong>{{ t('plugins.fields.icon') }}</strong>
            <p class="meta-icon">{{ textOrEmpty(plugin?.icon) }}</p>
          </section>

          <section class="metadata-section">
            <strong>{{ t('plugins.fields.repo') }}</strong>
            <a v-if="plugin?.repo" :href="plugin.repo" target="_blank" rel="noreferrer" class="meta-link">{{ plugin.repo }}</a>
            <p v-else class="empty-val">{{ t('display.empty') }}</p>
          </section>

          <section class="metadata-section">
            <strong>{{ t('plugins.fields.homepage') }}</strong>
            <a v-if="plugin?.homepage" :href="plugin.homepage" target="_blank" rel="noreferrer" class="meta-link">{{ plugin.homepage }}</a>
            <p v-else class="empty-val">{{ t('display.empty') }}</p>
          </section>

          <section class="metadata-section">
            <strong>{{ t('plugins.fields.keywords') }}</strong>
            <div v-if="hasItems(plugin?.keywords)" class="tag-list">
              <AppTag v-for="keyword in plugin?.keywords" :key="keyword">{{ keyword }}</AppTag>
            </div>
            <p v-else class="empty-val">{{ t('display.empty') }}</p>
          </section>

          <section class="metadata-section">
            <strong>{{ t('plugins.fields.webhooks') }}</strong>
            <pre v-if="hasItems(plugin?.webhooks)" class="metadata-json">{{ safeJsonStringify(plugin?.webhooks ?? {}) }}</pre>
            <p v-else class="empty-val">{{ t('display.empty') }}</p>
          </section>

          <section class="metadata-section">
            <strong>{{ t('plugins.fields.screenshots') }}</strong>
            <div v-if="hasItems(plugin?.screenshots)" class="screenshot-list">
              <article v-for="screenshot in plugin?.screenshots" :key="screenshot.path" class="screenshot-item">
                <span class="ss-path">{{ t('plugins.fields.screenshotPath') }}：{{ screenshot.path }}</span>
                <span class="ss-alt">{{ t('plugins.fields.screenshotAlt') }}：{{ textOrEmpty(screenshot.alt) }}</span>
              </article>
            </div>
            <p v-else class="empty-val">{{ t('display.empty') }}</p>
          </section>
        </div>
      </details>
    </div>
  </section>
</template>

<style scoped lang="scss">

.plugin-detail-summary-panel {
  min-width: 0;
}

.plugin-detail-summary-stack,
.plugin-detail-detail-stack {
  display: grid;
  gap: 14px;
}

.plugin-detail-summary-stack {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px 18px;
}

.plugin-detail-summary-section {
  display: grid;
  align-content: start;
  gap: 12px;
}

.plugin-detail-summary-section h3 {
  margin: 0;
  color: var(--text);
  font-size: 0.95rem;
  font-weight: 700;
  line-height: 1.35;
}

.plugin-detail-kv-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 14px;
  margin: 0;

  div {
    display: grid;
    min-width: 0;
    gap: 4px;
  }

  dt {
    color: var(--muted);
    font-size: 12px;
    font-weight: 500;
  }

  dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
    color: var(--text);
    font-size: 13px;
    font-weight: 500;
    line-height: 1.45;
  }
}

.metadata-section {
  display: grid;
  gap: 8px;

  strong {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--text);
  }

  p, a {
    margin: 0;
    word-break: break-word;
    font-size: 0.84rem;
  }
}

// Code sits on a white inset inside the gray box, without a second border.
.metadata-json {
  margin: 0;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-raised);
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.6;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.screenshot-list {
  display: grid;
}

.screenshot-item {
  display: grid;
  gap: 4px;
  padding-block: 8px;
  border-top: 1px solid var(--border);
  font-size: 13px;

  .ss-path {
    font-family: var(--font-mono);
    color: var(--muted);
  }
  .ss-alt {
    color: var(--text);
    font-weight: 550;
  }
}

.plugin-detail-disclosure {
  grid-column: 1 / -1;
  border-top: 1px solid var(--border);
  padding-top: 14px;

  summary {
    display: flex;
    align-items: center;
    gap: 10px;
    border-radius: var(--radius-xs);
    cursor: pointer;
    color: var(--text);
    font-weight: 700;
    font-size: 14px;
    list-style: none;

    &::-webkit-details-marker {
      display: none;
    }

    &:focus-visible {
      outline: 2px solid var(--focus);
      outline-offset: 2px;
    }
  }

  &[open] summary {
    margin-bottom: 12px;
  }
}

.plugin-detail-disclosure__chevron {
  color: var(--muted);
  transition: transform var(--motion-fast) var(--motion-easing);
}

.plugin-detail-disclosure[open] .plugin-detail-disclosure__chevron {
  transform: rotate(180deg);
}

@media (prefers-reduced-motion: reduce) {
  .plugin-detail-disclosure__chevron { transition: none; }
}
</style>
