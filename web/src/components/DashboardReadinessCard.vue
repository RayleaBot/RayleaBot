<script setup lang="ts">
import type { StatusType } from '@/lib/display'
import { t } from '@/i18n'

defineProps<{
  sectionTitle: string
  checkItems: Array<{ key: string; value: string; status: StatusType }>
  readinessNoteText: string
  reasonCodesLabel: string
  visibleReasonCodes: string[]
  readinessIssues: Array<{
    code: string
    severity: 'ok' | 'warning' | 'error'
    summary: string
    remediation?: string
  }>
  issuesExpanded: boolean
  expandIssuesText: string
  collapseIssuesText: string
}>()

defineEmits<{
  'toggle-issues': []
}>()

function getCheckIcon(status: StatusType): string {
  const map: Record<StatusType, string> = {
    success: '✅',
    warning: '⚠',
    danger: '❌',
    muted: '—',
  }
  return map[status]
}
</script>

<template>
  <el-card>
    <template #header>
      <div class="card-header">
        <span>{{ sectionTitle }}</span>
      </div>
    </template>

    <div v-if="checkItems.length" class="readiness-checks">
      <div
        v-for="item in checkItems"
        :key="item.key"
        :class="['readiness-check', `readiness-check--${item.status}`]"
      >
        <div class="readiness-check__header">
          <span class="readiness-check__icon">{{ getCheckIcon(item.status) }}</span>
          <span class="readiness-check__name">{{ item.key }}</span>
        </div>
        <div class="readiness-check__value">{{ item.value }}</div>
      </div>
    </div>

    <el-empty v-else :description="t('display.empty')" />

    <div v-if="readinessNoteText" class="readiness-note">
      <small style="color: var(--muted);">
        {{ readinessNoteText }}
      </small>
    </div>

    <div v-if="visibleReasonCodes.length" style="margin-top: 14px;">
      <small style="color: var(--muted);">{{ reasonCodesLabel }}: {{ visibleReasonCodes.join(', ') }}</small>
    </div>

    <div v-if="readinessIssues.length" class="issues-list" :class="{ 'issues-list--collapsed': !issuesExpanded && readinessIssues.length > 3 }">
      <div
        v-for="issue in readinessIssues"
        :key="`${issue.code}-${issue.summary}`"
        :class="['issue-alert-card', { 'issue-alert-card--warning': issue.severity === 'warning' }]"
      >
        <div class="issue-alert-card__header">
          <el-tag :type="issue.severity === 'error' ? 'danger' : issue.severity === 'warning' ? 'warning' : 'success'" size="small">
            {{ issue.code }}
          </el-tag>
          <span class="issue-alert-card__summary">{{ issue.summary }}</span>
        </div>
        <div v-if="issue.remediation" class="issue-alert-card__remediation">
          {{ issue.remediation }}
        </div>
      </div>
    </div>

    <div v-if="readinessIssues.length > 3" class="issues-toggle">
      <el-button size="small" text @click="$emit('toggle-issues')">
        {{ issuesExpanded ? collapseIssuesText : expandIssuesText }}
      </el-button>
    </div>
  </el-card>
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
</style>
