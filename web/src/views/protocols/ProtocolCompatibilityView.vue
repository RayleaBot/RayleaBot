<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { ONEBOT11_PROTOCOL_NAME } from '@/lib/protocols'
import { useProtocolCompatibilityStore } from '@/stores/protocol-compatibility'
import { useProtocolsStore } from '@/stores/protocols'

const protocolsStore = useProtocolsStore()
const compatibilityStore = useProtocolCompatibilityStore()

const {
  error: protocolsError,
  loading: protocolsLoading,
  snapshot,
} = storeToRefs(protocolsStore)
const {
  error: compatibilityError,
  loading: compatibilityLoading,
  matrix,
} = storeToRefs(compatibilityStore)

const selectedCategory = ref<string>('all')
const compatibilitySearch = ref('')

const transportLabelMap = {
  reverse_ws: t('config.sections.onebotReverseWs'),
  forward_ws: t('config.sections.onebotForwardWs'),
  http_api: t('config.sections.onebotHttpApi'),
  webhook: t('config.sections.onebotWebhook'),
} as const

const pageLoading = computed(() => protocolsLoading.value || compatibilityLoading.value)
const pageError = computed(() => protocolsError.value || compatibilityError.value)
const matrixSections = computed(() => matrix.value?.categories ?? [])
const categoryOptions = computed(() => [
  { label: '全部能力', value: 'all' },
  ...matrixSections.value.map((section) => ({ label: section.title, value: section.key })),
])
const filteredMatrixSections = computed(() => {
  const query = compatibilitySearch.value.trim().toLocaleLowerCase('zh-CN')
  return matrixSections.value
    .filter((section) => selectedCategory.value === 'all' || section.key === selectedCategory.value)
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => (
        !query
        || item.label.toLocaleLowerCase('zh-CN').includes(query)
        || item.key.toLocaleLowerCase('zh-CN').includes(query)
        || item.summary.toLocaleLowerCase('zh-CN').includes(query)
      )),
    }))
    .filter((section) => section.items.length > 0)
})
const currentProvider = computed(() => snapshot.value?.provider ?? 'unknown')
const currentProviderLabel = computed(() => formatProvider(currentProvider.value))
const currentTransportText = computed(() => joinTransportLabels(snapshot.value?.active_transports))
const configuredTransportText = computed(() => joinTransportLabels(snapshot.value?.configured_transports))
const currentTransportSummary = computed(() => snapshot.value?.summary ?? t('display.empty'))
const pageErrorToast = computed(() => (
  pageError.value && matrixSections.value.length > 0
    ? {
        key: `protocol-compatibility-error:${pageError.value}`,
        level: 'error' as const,
        message: pageError.value,
      }
    : null
))

useToastFeedback(pageErrorToast)

async function loadPage() {
  try {
    await Promise.all([
      protocolsStore.refresh(),
      compatibilityStore.refresh(),
    ])
  } catch {
    // store error state drives the page
  }
}

watch(() => snapshot.value?.provider, (next, previous) => {
  if (!next || !previous || next === previous) {
    return
  }
  void compatibilityStore.refresh().catch(() => undefined)
})

onMounted(() => {
  void loadPage()
})

function formatProvider(provider?: string) {
  switch (provider) {
    case 'standard':
      return 'Standard'
    case 'napcat':
      return 'NapCat'
    case 'luckylillia':
      return 'LuckyLillia'
    default:
      return t('protocols.unknownValue')
  }
}

function getTransportLabel(transport?: string) {
  if (!transport) {
    return t('display.empty')
  }
  return transportLabelMap[transport as keyof typeof transportLabelMap] ?? transport
}

function joinTransportLabels(transports?: readonly string[]) {
  if (!transports?.length) {
    return t('display.empty')
  }
  return transports.map((transport) => getTransportLabel(transport)).join(' / ')
}

function formatSupport(status?: string) {
  return status === 'supported'
    ? t('protocols.compatibilitySupported')
    : t('protocols.compatibilityUnsupported')
}

function supportClass(status?: string) {
  return status === 'supported' ? 'is-supported' : 'is-unsupported'
}

function providerColumnClass(provider: string) {
  if (currentProvider.value === 'unknown') {
    return {}
  }
  return {
    'is-current-provider': currentProvider.value === provider,
  }
}
</script>

<template>
  <AppPage :title="t('protocols.compatibilityTitle')" :description="t('protocols.compatibilitySubtitle')" width="detail">
    <div class="protocol-compatibility-page" data-testid="protocol-compatibility-page">
      <section class="protocol-overview-band" aria-label="协议运行摘要">
        <div class="protocol-overview-item">
          <span>{{ t('protocols.overviewTitle') }}</span>
          <strong>{{ currentProviderLabel }}</strong>
          <small>{{ ONEBOT11_PROTOCOL_NAME }} · {{ currentTransportSummary }}</small>
        </div>
        <div class="protocol-overview-item">
          <span>{{ t('protocols.activeTransportLabel') }}</span>
          <strong>{{ currentTransportText }}</strong>
          <small>{{ t('protocols.configuredTransportLabel') }}：{{ configuredTransportText }}</small>
        </div>
        <div class="protocol-overview-item">
          <span>{{ t('protocols.compatibilityTransportSummary') }}</span>
          <strong>{{ matrixSections.length }}</strong>
          <small>{{ t('protocols.compatibilityMatrixHint') }}</small>
        </div>
      </section>

      <RetryPanel
        v-if="pageError && matrixSections.length === 0"
        :title="t('protocols.compatibilityTitle')"
        :description="pageError"
        :loading="pageLoading"
        @retry="loadPage"
      />

      <section v-else class="protocol-compatibility-surface">
        <div class="protocol-compatibility-toolbar">
          <a-select
            v-model:value="selectedCategory"
            :options="categoryOptions"
            aria-label="能力分类"
          />
          <a-input
            v-model:value="compatibilitySearch"
            allow-clear
            aria-label="筛选兼容能力"
            placeholder="筛选能力名称、标识或说明"
          />
        </div>

        <div v-if="filteredMatrixSections.length > 0" class="protocol-compatibility-table-wrap">
          <table class="protocol-compatibility-table">
            <thead>
              <tr>
                <th>{{ t('protocols.compatibilityCapability') }}</th>
                <th>{{ '分类' }}</th>
                <th :class="providerColumnClass('standard')">Standard</th>
                <th :class="providerColumnClass('napcat')">NapCat</th>
                <th :class="providerColumnClass('luckylillia')">LuckyLillia</th>
                <th>{{ t('protocols.compatibilitySummary') }}</th>
              </tr>
            </thead>
            <tbody
              v-for="section in filteredMatrixSections"
              :key="section.key"
              :data-testid="`protocol-compatibility-${section.key}`"
            >
              <tr v-for="item in section.items" :key="item.key">
                <th scope="row" class="protocol-compatibility-table__capability">
                  <div class="protocol-compatibility-table__label">{{ item.label }}</div>
                  <code>{{ item.key }}</code>
                </th>
                <td class="protocol-compatibility-table__category">{{ section.title }}</td>
                <td :class="providerColumnClass('standard')">
                  <span class="protocol-support-pill" :class="supportClass(item.support.standard)">
                    {{ formatSupport(item.support.standard) }}
                  </span>
                </td>
                <td :class="providerColumnClass('napcat')">
                  <span class="protocol-support-pill" :class="supportClass(item.support.napcat)">
                    {{ formatSupport(item.support.napcat) }}
                  </span>
                </td>
                <td :class="providerColumnClass('luckylillia')">
                  <span class="protocol-support-pill" :class="supportClass(item.support.luckylillia)">
                    {{ formatSupport(item.support.luckylillia) }}
                  </span>
                </td>
                <td class="protocol-compatibility-table__summary">{{ item.summary }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="protocol-compatibility-empty" role="status">
          没有匹配的兼容能力。
        </div>
      </section>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.protocol-compatibility-page {
  display: grid;
  gap: var(--app-layout-gap);
}

.protocol-overview-band {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
}

.protocol-overview-item {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 16px;
}

.protocol-overview-item + .protocol-overview-item {
  border-inline-start: 1px solid var(--border);
}

.protocol-overview-item span,
.protocol-overview-item small {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.protocol-overview-item strong {
  color: var(--text);
  font-size: 18px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.protocol-compatibility-surface {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
}

.protocol-compatibility-toolbar {
  display: grid;
  grid-template-columns: minmax(160px, 220px) minmax(220px, 1fr);
  gap: 12px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
}

.protocol-compatibility-table-wrap {
  overflow-x: auto;
}

.protocol-compatibility-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  min-width: 980px;
}

.protocol-compatibility-table th,
.protocol-compatibility-table td {
  border-bottom: 1px solid var(--app-border);
  padding: 14px 12px;
  text-align: left;
  vertical-align: top;
}

.protocol-compatibility-table thead th {
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
  background: var(--surface-soft);
}

.protocol-compatibility-table__capability {
  width: 220px;
}

.protocol-compatibility-table__label {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 6px;
}

.protocol-compatibility-table__summary {
  color: var(--muted);
  min-width: 260px;
}

.protocol-compatibility-table__category {
  min-width: 120px;
  color: var(--muted);
}

.protocol-compatibility-table code {
  color: var(--app-text-secondary);
  font-size: 12px;
}

.protocol-support-pill {
  align-items: center;
  border-radius: 999px;
  border: 1px solid var(--app-border);
  display: inline-flex;
  font-size: 12px;
  font-weight: 600;
  justify-content: center;
  min-width: 76px;
  padding: 4px 10px;
}

.protocol-support-pill.is-supported {
  background: var(--success-soft);
  border-color: color-mix(in srgb, var(--success) 32%, var(--border));
  color: var(--text-success);
}

.protocol-support-pill.is-unsupported {
  background: var(--surface-soft);
  border-color: var(--border);
  color: var(--muted);
}

.is-current-provider {
  background: var(--surface-accent);
}

.protocol-compatibility-empty {
  padding: 32px 16px;
  color: var(--muted);
  text-align: center;
  font-size: 14px;
}

@media (max-width: 960px) {
  .protocol-overview-band {
    grid-template-columns: 1fr;
  }

  .protocol-overview-item + .protocol-overview-item {
    border-inline-start: 0;
    border-top: 1px solid var(--border);
  }
}

@media (max-width: 639px) {
  .protocol-compatibility-toolbar {
    grid-template-columns: 1fr;
  }
}
</style>
