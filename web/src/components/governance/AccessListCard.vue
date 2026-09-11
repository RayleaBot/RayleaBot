<script setup lang="ts">
import { computed } from 'vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppHelp from '@/components/AppHelp.vue'
import AppTag from '@/components/AppTag.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppInput from '@/components/AppInput.vue'
import AppDataTable from '@/components/AppDataTable.vue'
import AppButton from '@/components/AppButton.vue'
import AppCard from '@/components/AppCard.vue'
import GovernanceScopeEditor from '@/components/governance/GovernanceScopeEditor.vue'
import { governanceEntryKey, governanceScopeLabel } from '@/lib/governance-scope'
import { formatDateTime } from '@/lib/format'
import { notifyError, notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import type { BlacklistEntry, GovernanceEntryType, GovernanceBlacklistResponse } from '@/types/api'
import type { AccessListEditor } from './useAccessListEditor'

defineProps<{
  kind: 'blacklist' | 'whitelist'
  data: GovernanceBlacklistResponse | null
  loading: boolean
  loadingMore: boolean
  editor: AccessListEditor
}>()
const emit = defineEmits<{ remove: [entry: BlacklistEntry]; more: [] }>()

const scopeOptions = computed(() => [
  { label: t('accessLists.scopes.user'), value: 'user' },
  { label: t('accessLists.scopes.group'), value: 'group' },
])

const scopeFilterOptions = computed(() => [
  { label: t('accessLists.filters.all'), value: 'all' },
  { label: t('accessLists.scopes.user'), value: 'user' },
  { label: t('accessLists.scopes.group'), value: 'group' },
])

const tableColumns = computed(() => [
  { label: t('accessLists.namespace.label'), key: 'namespace', width: 230 },
  { label: t('accessLists.table.columns.type'), key: 'type', width: 120, align: 'center' as const },
  { label: t('accessLists.table.columns.targetId'), key: 'targetId', width: 180 },
  { label: t('accessLists.table.columns.reason'), key: 'reason' },
  { label: t('accessLists.table.columns.createdAt'), key: 'createdAt', width: 170 },
  { label: t('accessLists.table.columns.actions'), key: 'actions', width: 120, align: 'center' as const },
])

function getEntryTypeLabel(type: GovernanceEntryType) {
  return type === 'user' ? t('accessLists.scopes.user') : t('accessLists.scopes.group')
}

function getEntryTypeTagColor(type: GovernanceEntryType) {
  return type === 'user' ? 'info' : 'neutral'
}

async function copyTargetId(targetId: string) {
  try {
    await navigator.clipboard.writeText(targetId)
    notifySuccess(t('accessLists.actions.copyTargetId'))
  } catch {
    notifyError(t('ui.clipboard.copyFailed'))
  }
}

</script>

<template>
  <AppCard
    borderless
    class="access-lists-card"
    :loading="loading && !data"
  >
    <div :data-testid="`access-lists-${kind}-card`" class="access-lists-card-content">
      <div class="access-lists-card-header">
        <div class="access-lists-card-header__copy">
          <div class="access-lists-card-header__title-row">
            <strong>{{ t(`accessLists.cards.${kind}Title`) }}</strong>
            <AppHelp :label="t(`accessLists.cards.${kind}Help`)" :description="t(`accessLists.cards.${kind}Description`)" />
          </div>
        </div>
        <div class="access-lists-card-header__meta">
          <span class="access-lists-card-header__count">{{ data?.entry_count ?? 0 }}</span>
          <slot name="policy" />
        </div>
      </div>

      <slot name="notice" />

      <div class="access-lists-toolbar">
        <div class="access-lists-toolbar__row">
          <div class="toolbar-left-group">
            <AppSelect
              v-model="editor.scopeFilter"
              :options="scopeFilterOptions"
              class="access-lists-toolbar__filter"
              :aria-label="t('accessLists.filters.all')"
            />
            <AppInput
              v-model="editor.searchQuery" :maxlength="200"
              :placeholder="t('accessLists.entryForm.searchPlaceholder')"
              class="access-lists-toolbar__search"
              allow-clear
              :data-testid="`${kind}-search-input`"
            >
              <template #prefix>
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted-svg">
                  <circle cx="11" cy="11" r="8"></circle>
                  <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
                </svg>
              </template>
            </AppInput>
          </div>
          <div class="access-lists-toolbar__actions">
            <span class="access-lists-toolbar__count">{{ t('accessLists.table.total', { total: data?.total ?? 0 }) }}</span>
            <AppButton variant="default" :data-testid="`access-lists-${kind}-add-btn`" :disabled="editor.isAdding" @click="editor.startAdd">
              {{ t('accessLists.actions.addEntry') }}
            </AppButton>
          </div>
        </div>
      </div>

      <AppDataTable
        class="access-lists-data-table app-data-table"
        :columns="tableColumns"
        :rows="editor.tableData"
        :min-width="760"
        :row-key="(row) => row.isDraft ? `draft-${kind}` : governanceEntryKey(row)"
        :loading="loading && !data"
      >
        <template #empty>
          <div class="access-lists-empty-container">
            <div class="empty-graphic">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="42" height="42" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="8" x2="12" y2="12"></line>
                <line x1="12" y1="16" x2="12.01" y2="16"></line>
              </svg>
            </div>
            <p class="empty-title">{{ t(`accessLists.empty.${kind}Title`) }}</p>
            <p class="empty-desc">{{ t(`accessLists.empty.${kind}Description`) }}</p>
          </div>
        </template>

        <template #cell="{ column, row: record }">
          <template v-if="record.isDraft">
            <template v-if="column.key === 'type'">
              <AppSelect
                v-model="editor.draft.entry_type"
                :options="scopeOptions"
                style="width: 100%"
                :data-testid="`${kind}-draft-type`"
                :aria-label="t('accessLists.table.columns.type')"
              />
            </template>

            <template v-else-if="column.key === 'namespace'">
              <GovernanceScopeEditor v-model="editor.draft.scope" />
              <span v-if="editor.draftErrors.scope" class="inline-error-text" role="alert">{{ editor.draftErrors.scope }}</span>
            </template>

            <template v-else-if="column.key === 'targetId'">
              <div class="inline-edit-cell">
                <AppInput
                  v-model="editor.draft.target_id"
                  :placeholder="t('accessLists.entryForm.placeholderTargetId')"
                  :aria-invalid="Boolean(editor.draftErrors.target_id) || undefined"
                  :aria-label="t('accessLists.table.columns.targetId')"
                  :aria-describedby="editor.draftErrors.target_id ? `${kind}-draft-target_id-error` : undefined"
                  :data-testid="`${kind}-draft-target-id`"
                  @input="editor.draftErrors.target_id = ''"
                />
                <div v-if="editor.draftErrors.target_id" :id="`${kind}-draft-target_id-error`" class="inline-error-text" role="alert">
                  {{ editor.draftErrors.target_id }}
                </div>
              </div>
            </template>

            <template v-else-if="column.key === 'reason'">
              <div class="inline-edit-cell">
                <AppInput
                  v-model="editor.draft.reason"
                  :placeholder="t('accessLists.entryForm.placeholderReason')"
                  :aria-invalid="Boolean(editor.draftErrors.reason) || undefined"
                  :aria-label="t('accessLists.table.columns.reason')"
                  :aria-describedby="editor.draftErrors.reason ? `${kind}-draft-reason-error` : undefined"
                  :data-testid="`${kind}-draft-reason`"
                  @input="editor.draftErrors.reason = ''"
                />
                <div v-if="editor.draftErrors.reason" :id="`${kind}-draft-reason-error`" class="inline-error-text" role="alert">
                  {{ editor.draftErrors.reason }}
                </div>
              </div>
            </template>

            <template v-else-if="column.key === 'createdAt'">
              <span class="text-muted-inline">-</span>
            </template>

            <template v-else-if="column.key === 'actions'">
              <div class="inline-actions">
                <AppButton
                  variant="link"
                  size="sm"
                  :loading="editor.adding"
                  :data-testid="`${kind}-draft-save`"
                  @click="editor.save"
                >
                  {{ t('accessLists.modal.save') }}
                </AppButton>
                <AppButton
                  variant="link"
                  size="sm"
                  class="text-muted-btn"
                  :data-testid="`${kind}-draft-cancel`"
                  @click="editor.cancel"
                >
                  {{ t('accessLists.modal.cancel') }}
                </AppButton>
              </div>
            </template>
          </template>

          <template v-else>
            <template v-if="column.key === 'type'">
              <AppTag :tone="getEntryTypeTagColor(record.entry_type)">
                {{ getEntryTypeLabel(record.entry_type) }}
              </AppTag>
            </template>

            <template v-else-if="column.key === 'namespace'">
              <span>{{ governanceScopeLabel(record.scope) }}</span>
            </template>

            <template v-else-if="column.key === 'targetId'">
              <button
                type="button"
                class="target-id-chip copyable-text mono-text"
                :aria-label="`${t('accessLists.actions.copyTargetId')} ${record.target_id}`"
                @click="copyTargetId(record.target_id)"
              >
                <span class="chip-dot" :class="kind === 'whitelist' ? 'font-dot-success' : 'font-dot-danger'"></span>
                <span class="chip-text">{{ record.target_id }}</span>
                <span class="copy-icon-hover">
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                  </svg>
                </span>
              </button>
            </template>

            <template v-else-if="column.key === 'reason'">
              <span class="cell-reason">{{ record.reason }}</span>
            </template>

            <template v-else-if="column.key === 'createdAt'">
              <span>{{ formatDateTime(record.created_at) }}</span>
            </template>

            <template v-else-if="column.key === 'actions'">
              <AppButton variant="destructive" size="sm" class="remove-btn" @click="emit('remove', record)">{{ t('accessLists.entryForm.remove') }}</AppButton>
            </template>
          </template>
        </template>
      </AppDataTable>
      <AppCollectionPagination :loaded="editor.filteredEntries.length" :total="data?.total ?? 0" :next-cursor="data?.next_cursor" :loading="loadingMore || loading" @more="emit('more')" />
    </div>
  </AppCard>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;

.access-lists-card {
  border-radius: var(--radius-lg);
  background: var(--surface);
  border: 1px solid var(--border);
  transition: border-color 150ms ease;
}

:deep(.access-lists-card) {
  box-shadow: none;
}

.access-lists-card-content {
  display: grid;
  gap: 20px;
}

.access-lists-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.access-lists-card-header__copy {
  display: grid;
  gap: 4px;
}

.access-lists-card-header__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.access-lists-card-header__copy strong {
  font-size: 1.15rem;
  font-weight: 700;
  line-height: 1.2;
  color: var(--fg);
}

.access-lists-card-header__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.access-lists-card-header__count {
  font-size: 1.65rem;
  font-weight: 800;
  line-height: 1;
  color: var(--fg);
  letter-spacing: -0.02em;
}

.access-lists-toolbar {
  display: grid;
  gap: 12px;
}

.access-lists-toolbar__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.toolbar-left-group {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  max-width: 400px;
}

.access-lists-toolbar__filter {
  width: 110px;
  flex-shrink: 0;
}

.access-lists-toolbar__search {
  flex: 1;
}

.access-lists-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.access-lists-toolbar__count {
  font-size: 0.82rem;
  color: var(--muted);
}

.access-lists-data-table {
  border-radius: var(--radius-lg);
  overflow: hidden;
  border: 1px solid var(--border);
}

.access-lists-data-table :deep(thead > tr > th) {
  background: color-mix(in srgb, var(--surface-accent) 25%, var(--surface));
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--fg);
  border-bottom: 1px solid var(--border);
}

.access-lists-data-table :deep(tbody tr:hover > td) {
  background: var(--surface-accent) !important;
}

.target-id-chip {
  appearance: none;
  border: 1px solid color-mix(in srgb, var(--accent) 15%, var(--border));
  background: color-mix(in srgb, var(--accent) 5%, var(--surface));
  padding: 4px 10px;
  border-radius: 20px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--fg);
  transition: border-color 150ms ease, background-color 150ms ease, color 150ms ease;
  font-size: 0.85rem;
  font-weight: 600;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;

  .chip-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .font-dot-success {
    background-color: var(--success);
  }

  .font-dot-danger {
    background-color: var(--danger);
  }

  .chip-text {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .copy-icon-hover {
    color: var(--muted);
    opacity: 0;
    width: 12px;
    margin-left: 2px;
    transition: opacity var(--motion-fast) var(--motion-easing);
    display: inline-flex;
    align-items: center;
  }

  &:hover {
    border-color: var(--accent);
    background: color-mix(in srgb, var(--accent) 12%, var(--surface));

    .copy-icon-hover {
      opacity: 1;
    }
  }

  &:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: var(--focus-outline-offset);
  }
}

.cell-reason {
  font-size: 0.88rem;
  color: var(--fg-light, var(--fg));
}

.remove-btn {
  font-size: 0.85rem;
  font-weight: 500;
  padding: 0 4px;

  &:hover {
    color: var(--danger) !important;
  }
}

.inline-edit-cell {
  display: grid;
  gap: 4px;
  width: 100%;
}

.inline-error-text {
  font-size: 13px;
  color: var(--danger);
  text-align: left;
  line-height: 1.2;
}

.inline-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.text-muted-btn {
  color: var(--muted) !important;

  &:hover {
    color: var(--fg) !important;
  }
}

.text-muted-inline {
  color: var(--muted);
}

.text-muted-svg {
  color: var(--muted);
  opacity: 0.7;
}

.access-lists-empty-container {
  padding: 44px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  background: color-mix(in srgb, var(--surface-accent) 15%, transparent);
  border-radius: var(--radius-lg);
  border: 1px dashed var(--border);
  margin: 12px 0;

  .empty-graphic {
    color: var(--muted);
    opacity: 0.45;
    margin-bottom: 12px;
  }

  .empty-title {
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--fg);
    margin: 0 0 4px;
  }

  .empty-desc {
    font-size: 0.82rem;
    color: var(--muted);
    margin: 0;
    max-width: 280px;
  }
}

.mono-text {
  font-family: var(--font-mono);
}

@media (max-width: #{bp.$tablet}) {
  .access-lists-card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .access-lists-card-header__meta {
    width: 100%;
  }

  .access-lists-toolbar__row {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .toolbar-left-group {
    width: 100%;
    max-width: 100%;
  }

  .access-lists-toolbar__actions {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
