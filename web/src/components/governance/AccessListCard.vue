<script setup lang="ts">
import { computed } from 'vue'
import { CopyIcon, PlusIcon, RotateCwIcon, SearchIcon } from '@lucide/vue'
import AppAlert from '@/components/AppAlert.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppHelp from '@/components/AppHelp.vue'
import AppTag from '@/components/AppTag.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppInput from '@/components/AppInput.vue'
import AppDataTable from '@/components/AppDataTable.vue'
import AppButton from '@/components/AppButton.vue'
import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import GovernanceScopeEditor from '@/components/governance/GovernanceScopeEditor.vue'
import { governanceEntryKey, governanceScopeLabel } from '@/lib/governance-scope'
import { formatDateTime } from '@/lib/format'
import { copyText } from '@/adapter/clipboard'
import { t } from '@/i18n'
import type { BlacklistEntry, GovernanceEntryType, GovernanceBlacklistResponse } from '@/types/api'
import type { AccessListEditor } from './useAccessListEditor'

const props = defineProps<{
  kind: 'blacklist' | 'whitelist'
  data: GovernanceBlacklistResponse | null
  loading: boolean
  loadingMore: boolean
  // The last read of this list failed. Without data the list is unknown, so neither its entries nor its policy are shown.
  error?: string | null
  editor: AccessListEditor
}>()
const emit = defineEmits<{ remove: [entry: BlacklistEntry]; more: []; retry: [] }>()

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

// entry_count is the whole list; total is what the current search and type filter match.
const countLabel = computed(() => {
  const total = props.data?.total ?? 0
  const count = props.data?.entry_count ?? total
  return total === count ? t('accessLists.table.total', { total }) : t('accessLists.table.matched', { total, count })
})
const hasUnmatchedEntries = computed(() => (props.data?.entry_count ?? 0) > 0)

function getEntryTypeLabel(type: GovernanceEntryType) {
  return type === 'user' ? t('accessLists.scopes.user') : t('accessLists.scopes.group')
}

function copyTargetId(targetId: string) {
  return copyText(targetId, t('accessLists.feedback.targetIdCopied'))
}

function clearFilters() {
  props.editor.searchQuery = ''
  props.editor.scopeFilter = 'all'
}

</script>

<template>
  <AppCard :loading="loading && !data">
    <div :data-testid="`access-lists-${kind}-card`" class="access-lists-card-content">
      <div class="access-lists-card-header">
        <div class="access-lists-card-header__title-row">
          <h2>{{ t(`accessLists.cards.${kind}Title`) }}</h2>
          <AppHelp :label="t(`accessLists.cards.${kind}Help`)" :description="t(`accessLists.cards.${kind}Description`)" />
        </div>
        <div v-if="data && $slots.policy" class="access-lists-card-header__meta">
          <slot name="policy" />
        </div>
      </div>

      <RetryPanel
        v-if="error && !data"
        :title="t(`accessLists.errors.${kind}LoadFailed`)"
        :description="error"
        :loading="loading"
        @retry="emit('retry')"
      />

      <template v-else>
        <slot name="notice" />

        <!-- A failed refresh keeps the last read entries on screen and says so. -->
        <AppAlert
          v-if="error"
          tone="danger"
          :data-testid="`access-lists-${kind}-refresh-error`"
          :title="t('accessLists.errors.refreshFailed')"
          :description="error"
        >
          <template #action>
            <AppButton size="sm" :loading="loading" @click="emit('retry')">
              <template #icon><RotateCwIcon /></template>
              {{ t('ui.retry') }}
            </AppButton>
          </template>
        </AppAlert>

        <div class="access-lists-toolbar">
          <div class="toolbar-left-group">
            <AppSelect
              v-model="editor.scopeFilter"
              :options="scopeFilterOptions"
              wrapper-class="access-lists-toolbar__filter"
              :aria-label="t('accessLists.filters.type')"
            />
            <AppInput
              v-model="editor.searchQuery" :maxlength="200"
              :placeholder="t('accessLists.entryForm.searchPlaceholder')"
              wrapper-class="access-lists-toolbar__search"
              allow-clear
              :data-testid="`${kind}-search-input`"
            >
              <template #prefix>
                <SearchIcon :size="16" class="access-lists-toolbar__search-icon" aria-hidden="true" />
              </template>
            </AppInput>
          </div>
          <div class="access-lists-toolbar__actions">
            <span class="access-lists-toolbar__count" :data-testid="`access-lists-${kind}-count`">{{ countLabel }}</span>
            <AppButton variant="default" :data-testid="`access-lists-${kind}-add-btn`" :disabled="editor.isAdding" @click="editor.startAdd">
              <template #icon><PlusIcon /></template>
              {{ t('accessLists.actions.addEntry') }}
            </AppButton>
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
            <AppEmptyState
              v-if="hasUnmatchedEntries"
              icon="search"
              :title="t('accessLists.empty.filteredTitle')"
              :description="t('accessLists.empty.filteredDescription')"
              :action-label="t('accessLists.empty.clearFilters')"
              @action="clearFilters"
            />
            <AppEmptyState v-else icon="box" :title="t(`accessLists.empty.${kind}Title`)" :description="t(`accessLists.empty.${kind}Description`)" />
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
                <AppTag>{{ getEntryTypeLabel(record.entry_type) }}</AppTag>
              </template>

              <template v-else-if="column.key === 'namespace'">
                <span>{{ governanceScopeLabel(record.scope) }}</span>
              </template>

              <template v-else-if="column.key === 'targetId'">
                <button
                  type="button"
                  class="target-id-chip copyable-text mono-text"
                  :aria-label="t('accessLists.actions.copyTargetId', { target: record.target_id })"
                  @click="copyTargetId(record.target_id)"
                >
                  <span class="chip-text">{{ record.target_id }}</span>
                  <CopyIcon class="copy-icon-hover" :size="12" aria-hidden="true" />
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
      </template>
    </div>
  </AppCard>
</template>

<style scoped lang="scss">

.access-lists-card-content {
  display: grid;
  gap: 20px;
}

.access-lists-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 32px;
}

.access-lists-card-header__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.access-lists-card-header__title-row h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  line-height: 1.3;
  color: var(--text);
}

.access-lists-card-header__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.access-lists-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.toolbar-left-group {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.toolbar-left-group :deep(.access-lists-toolbar__filter) {
  flex: none;
  width: 140px;
}

.toolbar-left-group :deep(.access-lists-toolbar__search) {
  flex: none;
  width: 280px;
}

.access-lists-toolbar__search-icon {
  color: var(--muted);
}

.access-lists-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.access-lists-toolbar__count {
  font-size: 13px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

// The copyable ID reads as text; hovering reveals the copy icon on a quiet fill.
.target-id-chip {
  appearance: none;
  border: 0;
  background: transparent;
  padding: 4px 8px;
  margin-inline-start: -8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  transition: background-color var(--motion-fast) var(--motion-easing);
  font-size: 13px;
  font-weight: 600;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;

  .chip-text {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .copy-icon-hover {
    flex: none;
    color: var(--muted);
    opacity: 0;
    transition: opacity var(--motion-fast) var(--motion-easing);
  }

  &:hover,
  &:focus-visible {
    background: var(--nav-hover);

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
  font-size: 13px;
  color: var(--muted);
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

.mono-text {
  font-family: var(--font-mono);
}
</style>
