<script setup lang="ts">
import { computed } from 'vue'
import { ChevronDownIcon } from '@lucide/vue'

import AppTag from '@/components/AppTag.vue'
import { t } from '@/i18n'
import { formatPluginVersion, getPluginInstallMethodLabel, getPluginTrustLabel } from '@/lib/display'
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
        { key: 'method', label: t('plugins.fields.sourceRef'), value: getPluginInstallMethodLabel(plugin?.source) },
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

// Manifest fields the plugin declares are listed; the ones it leaves out share a single line.
const metadataFields = computed(() => {
  const plugin = props.plugin
  return [
    { key: 'description', label: t('plugins.fields.description'), present: Boolean(plugin?.description?.trim()) },
    { key: 'icon', label: t('plugins.fields.icon'), present: Boolean(plugin?.icon?.trim()) },
    { key: 'repo', label: t('plugins.fields.repo'), present: Boolean(plugin?.repo) },
    { key: 'homepage', label: t('plugins.fields.homepage'), present: Boolean(plugin?.homepage) },
    { key: 'keywords', label: t('plugins.fields.keywords'), present: hasItems(plugin?.keywords) },
    { key: 'webhooks', label: t('plugins.fields.webhooks'), present: hasItems(plugin?.webhooks) },
    { key: 'screenshots', label: t('plugins.fields.screenshots'), present: hasItems(plugin?.screenshots) },
  ]
})
const declared = computed(() => new Set(metadataFields.value.filter(field => field.present).map(field => field.key)))
const undeclaredLabels = computed(() => metadataFields.value.filter(field => !field.present).map(field => field.label))

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
          <div v-if="section.key === 'runtime'" class="plugin-detail-kv-list__wide">
            <dt>{{ t('plugins.fields.events') }}</dt>
            <dd>
              <span v-if="hasItems(plugin?.events)" class="tag-list">
                <AppTag v-for="eventName in plugin?.events" :key="eventName">{{ eventName }}</AppTag>
              </span>
              <template v-else>{{ t('display.empty') }}</template>
            </dd>
          </div>
        </dl>
      </section>

      <details class="plugin-detail-disclosure">
        <summary>
          <span>{{ t('plugins.sections.details') }}</span>
          <ChevronDownIcon class="plugin-detail-disclosure__chevron" :size="16" aria-hidden="true" />
          <AppTag size="small">{{ t('plugins.sections.metadata') }}</AppTag>
        </summary>

        <dl class="plugin-detail-kv-list plugin-detail-metadata">
          <div v-if="declared.has('description')" class="plugin-detail-kv-list__wide">
            <dt>{{ t('plugins.fields.description') }}</dt>
            <dd>{{ plugin?.description }}</dd>
          </div>
          <div v-if="declared.has('icon')">
            <dt>{{ t('plugins.fields.icon') }}</dt>
            <dd class="is-mono">{{ plugin?.icon }}</dd>
          </div>
          <div v-if="declared.has('repo')">
            <dt>{{ t('plugins.fields.repo') }}</dt>
            <dd><a :href="plugin?.repo" target="_blank" rel="noreferrer" class="meta-link">{{ plugin?.repo }}</a></dd>
          </div>
          <div v-if="declared.has('homepage')">
            <dt>{{ t('plugins.fields.homepage') }}</dt>
            <dd><a :href="plugin?.homepage" target="_blank" rel="noreferrer" class="meta-link">{{ plugin?.homepage }}</a></dd>
          </div>
          <div v-if="declared.has('keywords')">
            <dt>{{ t('plugins.fields.keywords') }}</dt>
            <dd class="tag-list"><AppTag v-for="keyword in plugin?.keywords" :key="keyword">{{ keyword }}</AppTag></dd>
          </div>
          <div v-if="declared.has('webhooks')" class="plugin-detail-kv-list__wide">
            <dt>{{ t('plugins.fields.webhooks') }}</dt>
            <dd><pre class="metadata-json">{{ safeJsonStringify(plugin?.webhooks ?? {}) }}</pre></dd>
          </div>
          <div v-if="declared.has('screenshots')" class="plugin-detail-kv-list__wide">
            <dt>{{ t('plugins.fields.screenshots') }}</dt>
            <dd class="screenshot-list">
              <article v-for="screenshot in plugin?.screenshots" :key="screenshot.path" class="screenshot-item">
                <span class="ss-path">{{ screenshot.path }}</span>
                <span v-if="screenshot.alt?.trim()" class="ss-alt">{{ screenshot.alt }}</span>
              </article>
            </dd>
          </div>
          <div v-if="undeclaredLabels.length" class="plugin-detail-kv-list__wide">
            <dt>{{ t('plugins.sections.undeclared') }}</dt>
            <dd class="plugin-detail-metadata__undeclared">{{ undeclaredLabels.join('、') }}</dd>
          </div>
        </dl>
      </details>
    </div>
  </section>
</template>

<style scoped lang="scss">

.plugin-detail-summary-panel {
  min-width: 0;
}

.plugin-detail-summary-stack {
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

.plugin-detail-kv-list .plugin-detail-kv-list__wide {
  grid-column: 1 / -1;
}

.plugin-detail-kv-list dd.is-mono {
  font-family: var(--font-mono);
}

// The manifest fields span the whole panel, so they use three columns instead of two.
.plugin-detail-metadata {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.plugin-detail-metadata__undeclared {
  color: var(--muted) !important;
  font-weight: 400 !important;
}

.meta-link {
  color: var(--brand-foreground);
  overflow-wrap: anywhere;
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
  font-size: 13px;

  & + & {
    border-top: 1px solid var(--border);
  }


  .ss-path {
    font-family: var(--font-mono);
    color: var(--muted);
  }
  .ss-alt {
    color: var(--text);
    font-weight: 400;
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
