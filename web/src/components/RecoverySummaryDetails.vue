<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { getRecoveryStatusLabel } from '@/lib/display'
import { formatRelativeTime } from '@/lib/format'
import { t } from '@/i18n'
import type { RecoveryCompatibilitySummary, RecoveryCompatibilitySkippedPlugin } from '@/types/api'

type RecoveryFilter = 'all' | 'pending' | 'confirmed'

const selectedRecoveryReviewIds = defineModel<string[]>('selectedRecoveryReviewIds', { default: () => [] })

const props = withDefaults(defineProps<{
  recoverySummary: RecoveryCompatibilitySummary
  recoveryStatusLabel?: string
  showPluginLinks?: boolean
  showSelectionControls?: boolean
}>(), {
  recoveryStatusLabel: undefined,
  showPluginLinks: false,
  showSelectionControls: false,
})

defineEmits<{
  openPlugin: [pluginId: string]
}>()

const selectedFilter = ref<RecoveryFilter | null>(null)

const skippedPlugins = computed(() => props.recoverySummary.skipped_plugins ?? [])
const pendingSkippedPlugins = computed(() => skippedPlugins.value.filter((plugin) => plugin.review_status !== 'confirmed'))
const confirmedSkippedPlugins = computed(() => skippedPlugins.value.filter((plugin) => plugin.review_status === 'confirmed'))
const defaultFilter = computed<RecoveryFilter>(() => pendingSkippedPlugins.value.length > 0 ? 'pending' : 'all')
const activeFilter = computed<RecoveryFilter>({
  get: () => selectedFilter.value ?? defaultFilter.value,
  set: (value) => {
    selectedFilter.value = value
  },
})

const filteredSkippedPlugins = computed<RecoveryCompatibilitySkippedPlugin[]>(() => {
  switch (activeFilter.value) {
    case 'pending':
      return pendingSkippedPlugins.value
    case 'confirmed':
      return confirmedSkippedPlugins.value
    default:
      return skippedPlugins.value
  }
})

const recoveryStatusLabel = computed(() => props.recoveryStatusLabel ?? getRecoveryStatusLabel(props.recoverySummary.status))
const recoveryAuditEntries = computed(() => props.recoverySummary.audit ?? [])

watch([skippedPlugins, pendingSkippedPlugins, confirmedSkippedPlugins], ([all, pending, confirmed]) => {
  if (all.length === 0) {
    selectedFilter.value = null
    return
  }
  if (selectedFilter.value === 'pending' && pending.length === 0) {
    selectedFilter.value = null
  }
  if (selectedFilter.value === 'confirmed' && confirmed.length === 0) {
    selectedFilter.value = null
  }
})
</script>

<template>
  <div class="events-section">
    <div class="issue-alert-card" :class="{ 'issue-alert-card--warning': recoverySummary.status !== 'compatible' }">
      <div class="issue-alert-card__header">
        <a-tag :color="recoverySummary.status === 'blocked' ? 'error' : recoverySummary.status === 'compatible' ? 'success' : 'warning'">
          {{ recoveryStatusLabel }}
        </a-tag>
        <span class="issue-alert-card__summary">
          {{ recoverySummary.operation }} · {{ recoverySummary.phase }}
        </span>
      </div>
      <div class="issue-alert-card__remediation">
        core {{ recoverySummary.source_core_version ?? t('display.empty') }} -> {{ recoverySummary.target_core_version ?? t('display.empty') }}
      </div>
    </div>

    <div
      v-for="issue in recoverySummary.issues ?? []"
      :key="issue.code"
      class="issue-alert-card"
      :class="{ 'issue-alert-card--warning': issue.severity === 'warning' }"
    >
      <div class="issue-alert-card__header">
        <a-tag :color="issue.severity === 'error' ? 'error' : 'warning'">
          {{ issue.code }}
        </a-tag>
        <span class="issue-alert-card__summary">{{ issue.summary }}</span>
      </div>
      <div v-if="issue.remediation" class="issue-alert-card__remediation">
        {{ issue.remediation }}
      </div>
    </div>

    <div v-if="skippedPlugins.length" class="recovery-summary__toolbar">
      <small class="recovery-summary__section-label">{{ t('display.recoveryItems') }}</small>
      <div class="recovery-summary__filters">
        <a-button
          data-testid="recovery-filter-all"
          size="small"
          :type="activeFilter === 'all' ? 'primary' : 'default'"
          @click="activeFilter = 'all'"
        >
          {{ t('display.recoveryFilters.all') }}
        </a-button>
        <a-button
          data-testid="recovery-filter-pending"
          size="small"
          :type="activeFilter === 'pending' ? 'primary' : 'default'"
          @click="activeFilter = 'pending'"
        >
          {{ t('display.recoveryFilters.pending') }}
        </a-button>
        <a-button
          data-testid="recovery-filter-confirmed"
          size="small"
          :type="activeFilter === 'confirmed' ? 'primary' : 'default'"
          @click="activeFilter = 'confirmed'"
        >
          {{ t('display.recoveryFilters.confirmed') }}
        </a-button>
      </div>
    </div>

    <div
      v-for="plugin in filteredSkippedPlugins"
      :key="plugin.review_id"
      :data-testid="`recovery-plugin-card-${plugin.review_id}`"
      class="issue-alert-card issue-alert-card--warning"
    >
      <div class="issue-alert-card__header">
        <a-tag :color="plugin.review_status === 'confirmed' ? 'success' : 'warning'">
          {{ plugin.reason_code }}
        </a-tag>
        <a-button
          v-if="showPluginLinks"
          type="link"
          class="issue-alert-card__summary issue-alert-card__summary--link"
          :data-testid="`recovery-plugin-link-${plugin.plugin_id}`"
          @click="$emit('openPlugin', plugin.plugin_id)"
        >
          {{ plugin.plugin_id }}
        </a-button>
        <span v-else class="issue-alert-card__summary">
          {{ plugin.plugin_id }}
        </span>
        <a-tag :color="plugin.review_status === 'confirmed' ? 'success' : 'warning'">
          {{ plugin.review_status === 'confirmed' ? t('dashboard.recoveryConfirmed') : t('dashboard.recoveryPending') }}
        </a-tag>
      </div>
      <div class="issue-alert-card__remediation">{{ plugin.summary }}</div>
      <div v-if="plugin.manual_action" class="issue-alert-card__remediation">{{ plugin.manual_action }}</div>
      <div v-if="plugin.review_status === 'confirmed'" class="issue-alert-card__remediation">
        {{ t('dashboard.recoveryReviewedBy') }}：{{ plugin.reviewed_by || t('display.empty') }}
        · {{ t('dashboard.recoveryReviewedAt') }}：{{ plugin.reviewed_at ? formatRelativeTime(plugin.reviewed_at) : t('display.empty') }}
      </div>
      <div
        v-else-if="showSelectionControls"
        :data-testid="`recovery-confirm-checkbox-${plugin.review_id}`"
        class="recovery-summary__checkbox"
      >
        <a-checkbox-group v-model:value="selectedRecoveryReviewIds">
          <a-checkbox :value="plugin.review_id">
            {{ t('dashboard.recoveryConfirm') }}
          </a-checkbox>
        </a-checkbox-group>
      </div>
    </div>

    <div v-if="skippedPlugins.length && filteredSkippedPlugins.length === 0" class="readiness-note">
      <small style="color: var(--muted);">{{ t('display.recoveryFilterEmpty') }}</small>
    </div>

    <slot name="after-skipped-plugins" />

    <div v-if="recoverySummary.manual_actions?.length" class="recovery-summary__section">
      <small class="recovery-summary__section-label">{{ t('display.recoveryManualActions') }}</small>
      <ul class="recovery-summary__list">
        <li
          v-for="action in recoverySummary.manual_actions"
          :key="action"
          data-testid="recovery-manual-action"
        >
          {{ action }}
        </li>
      </ul>
    </div>

    <div v-if="recoverySummary.next_steps?.length" class="recovery-summary__section">
      <small class="recovery-summary__section-label">{{ t('display.recoveryNextSteps') }}</small>
      <ul class="recovery-summary__list">
        <li
          v-for="step in recoverySummary.next_steps"
          :key="step"
          data-testid="recovery-next-step"
        >
          {{ step }}
        </li>
      </ul>
    </div>

    <div v-if="recoveryAuditEntries.length" class="recovery-summary__section">
      <small class="recovery-summary__section-label">{{ t('display.recoveryHistory') }}</small>
      <div
        v-for="entry in recoveryAuditEntries"
        :key="`${entry.task_id}-${entry.created_at}`"
        data-testid="recovery-audit-entry"
        class="issue-alert-card"
      >
        <div class="issue-alert-card__header">
          <a-tag color="blue">{{ entry.operator_id }}</a-tag>
          <span class="issue-alert-card__summary">{{ formatRelativeTime(entry.created_at) }}</span>
        </div>
        <div class="issue-alert-card__remediation">{{ entry.note || t('display.empty') }}</div>
        <ul class="recovery-summary__list recovery-summary__list--compact">
          <li v-for="item in entry.items" :key="item.review_id">
            {{ item.plugin_id }} · {{ item.reason_code }}
          </li>
        </ul>
      </div>
    </div>
    <div v-else class="readiness-note">
      <small style="color: var(--muted);">{{ t('dashboard.recoveryAuditEmpty') }}</small>
    </div>
  </div>
</template>

<style scoped lang="scss">
.readiness-note {
  margin-top: 14px;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.04);
  border: 1px solid rgba(148, 163, 184, 0.18);
  line-height: 1.5;
}

.recovery-summary__toolbar,
.recovery-summary__section {
  display: grid;
  gap: 8px;
}

.recovery-summary__toolbar {
  grid-template-columns: minmax(0, 1fr);
}

.recovery-summary__section-label {
  color: var(--muted);
  display: block;
}

.recovery-summary__filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.recovery-summary__checkbox {
  margin-top: 10px;
}

.recovery-summary__list {
  margin: 0;
  padding-left: 18px;
  color: var(--muted);
  display: grid;
  gap: 6px;
}

.recovery-summary__list--compact {
  margin-top: 8px;
}
</style>
