<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'

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

const transportLabelMap = {
  reverse_ws: t('config.sections.onebotReverseWs'),
  forward_ws: t('config.sections.onebotForwardWs'),
  http_api: t('config.sections.onebotHttpApi'),
  webhook: t('config.sections.onebotWebhook'),
} as const

const pageLoading = computed(() => protocolsLoading.value || compatibilityLoading.value)
const pageError = computed(() => protocolsError.value || compatibilityError.value)
const matrixSections = computed(() => matrix.value?.categories ?? [])
const currentProvider = computed(() => snapshot.value?.provider ?? 'unknown')
const currentProviderLabel = computed(() => formatProvider(currentProvider.value))
const currentTransportText = computed(() => joinTransportLabels(snapshot.value?.active_transports))
const configuredTransportText = computed(() => joinTransportLabels(snapshot.value?.configured_transports))
const currentTransportSummary = computed(() => snapshot.value?.summary ?? t('display.empty'))

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
  <AppPage :title="t('protocols.compatibilityTitle')" :description="t('protocols.compatibilitySubtitle')">
    <template #extra>
      <a-button :loading="pageLoading" @click="loadPage">{{ t('dashboard.refresh') }}</a-button>
    </template>

    <div class="protocol-compatibility-page" data-testid="protocol-compatibility-page">
      <div class="protocol-overview-grid">
        <a-card :bordered="false" class="protocol-overview-card">
          <div class="protocol-overview-card__top">
            <span class="overview-label">{{ t('protocols.overviewTitle') }}</span>
            <a-tag color="blue">{{ ONEBOT11_PROTOCOL_NAME }}</a-tag>
          </div>
          <div class="protocol-overview-card__value-row">
            <strong>{{ currentProviderLabel }}</strong>
            <a-tag>{{ t('protocols.compatibilityCurrentProvider') }}</a-tag>
          </div>
          <p>{{ currentTransportSummary }}</p>
        </a-card>

        <a-card :bordered="false" class="protocol-overview-card">
          <div class="protocol-overview-card__top">
            <span class="overview-label">{{ t('protocols.activeTransportLabel') }}</span>
            <a-tag>{{ snapshot?.active_transports.length || 0 }}</a-tag>
          </div>
          <strong>{{ currentTransportText }}</strong>
          <p>{{ t('protocols.configuredTransportLabel') }}：{{ configuredTransportText }}</p>
        </a-card>

        <a-card :bordered="false" class="protocol-overview-card">
          <div class="protocol-overview-card__top">
            <span class="overview-label">{{ t('protocols.compatibilityTransportSummary') }}</span>
            <a-tag>{{ currentProviderLabel }}</a-tag>
          </div>
          <strong>{{ currentTransportSummary }}</strong>
          <p>{{ t('protocols.compatibilityMatrixHint') }}</p>
        </a-card>
      </div>

      <a-alert
        v-if="pageError && matrixSections.length > 0"
        type="error"
        show-icon
        :message="t('errors.common.actionFailed')"
        :description="pageError"
      />

      <RetryPanel
        v-if="pageError && matrixSections.length === 0"
        :title="t('protocols.compatibilityTitle')"
        :description="pageError"
        :loading="pageLoading"
        @retry="loadPage"
      />

      <div v-else class="protocol-compatibility-sections">
        <a-card
          v-for="section in matrixSections"
          :key="section.key"
          :bordered="false"
          class="protocol-compatibility-card"
          :data-testid="`protocol-compatibility-${section.key}`"
        >
          <div class="section-heading">
            <div>
              <h2>{{ section.title }}</h2>
              <p class="subtitle">{{ t('protocols.compatibilityMatrixHint') }}</p>
            </div>
          </div>

          <div class="protocol-compatibility-table-wrap">
            <table class="protocol-compatibility-table">
              <thead>
                <tr>
                  <th>{{ t('protocols.compatibilityCapability') }}</th>
                  <th :class="providerColumnClass('standard')">Standard</th>
                  <th :class="providerColumnClass('napcat')">NapCat</th>
                  <th :class="providerColumnClass('luckylillia')">LuckyLillia</th>
                  <th>{{ t('protocols.compatibilitySummary') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in section.items" :key="item.key">
                  <th scope="row" class="protocol-compatibility-table__capability">
                    <div class="protocol-compatibility-table__label">{{ item.label }}</div>
                    <code>{{ item.key }}</code>
                  </th>
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
        </a-card>
      </div>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.protocol-compatibility-page {
  display: grid;
  gap: var(--app-layout-gap);
}

.protocol-overview-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.protocol-overview-card :deep(.ant-card-body),
.protocol-compatibility-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.protocol-overview-card :deep(.ant-card-body) {
  padding: 14px;
}

.protocol-overview-card__top,
.protocol-overview-card__value-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.protocol-overview-card__value-row {
  align-items: flex-start;
}

.overview-label {
  color: var(--app-text-secondary);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.protocol-compatibility-sections {
  display: grid;
  gap: 12px;
}

.protocol-compatibility-table-wrap {
  overflow-x: auto;
}

.protocol-compatibility-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  min-width: 860px;
}

.protocol-compatibility-table th,
.protocol-compatibility-table td {
  border-bottom: 1px solid var(--app-border);
  padding: 14px 12px;
  text-align: left;
  vertical-align: top;
}

.protocol-compatibility-table thead th {
  color: var(--app-text-secondary);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
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
  color: var(--app-text-secondary);
  min-width: 260px;
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
  background: rgba(22, 163, 74, 0.08);
  border-color: rgba(22, 163, 74, 0.24);
  color: var(--text-success);
}

.protocol-support-pill.is-unsupported {
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.24);
  color: var(--app-text-secondary);
}

.is-current-provider {
  background: rgba(24, 144, 255, 0.08);
}

@media (max-width: 960px) {
  .protocol-overview-grid {
    grid-template-columns: 1fr;
  }
}
</style>
