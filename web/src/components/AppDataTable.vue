<script setup lang="ts" generic="T extends object">
import type { DataColumn } from './data-table'
import AppEmptyState from './AppEmptyState.vue'
import AppSkeleton from './AppSkeleton.vue'
withDefaults(defineProps<{
  rows: readonly T[]
  columns: readonly DataColumn[]
  rowKey: (row: T) => string
  loading?: boolean
  minWidth?: number
  label?: string
}>(), { label: '数据表格' })
defineSlots<{
  cell?: (props: { row: T; column: DataColumn; index: number }) => unknown
  empty?: () => unknown
}>()
function cellText(row: T, key: string) {
  const value = row[key as keyof T]
  return value === null || value === undefined ? '—' : String(value)
}
</script>
<template>
  <div class="app-data-table" :aria-busy="loading || undefined">
    <div class="app-data-table__scroller" :tabindex="minWidth ? 0 : undefined" :aria-label="label">
      <table :style="{ minWidth: minWidth ? minWidth + 'px' : undefined }">
        <caption class="sr-only">{{ label }}</caption>
        <thead>
          <tr><th v-for="column in columns" :key="column.key" scope="col" :style="{ width: column.width ? column.width + 'px' : undefined, textAlign: column.align }">{{ column.label }}</th></tr>
        </thead>
        <tbody>
          <tr v-if="loading && rows.length === 0"><td :colspan="columns.length"><AppSkeleton :rows="5" /></td></tr>
          <tr v-for="(row, index) in rows" v-else :key="rowKey(row)">
            <td v-for="column in columns" :key="column.key" :style="{ textAlign: column.align }">
              <slot name="cell" :row="row" :column="column" :index="index">{{ cellText(row, column.key) }}</slot>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="!loading && rows.length === 0" class="app-data-table__empty"><slot name="empty"><AppEmptyState title="暂无数据" /></slot></div>
  </div>
</template>
<style>
.app-data-table { min-width: 0; width: 100%; }
.app-data-table__scroller { overflow-x: auto; overscroll-behavior-x: contain; }
.app-data-table__scroller:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; border-radius: 4px; }
.app-data-table table { width: 100%; border-collapse: collapse; font-size: 13px; line-height: 1.6; }
.app-data-table th { padding: 12px 16px; border-bottom: 1px solid var(--border); background: var(--surface-soft); color: var(--muted); font-weight: 500; text-align: left; white-space: nowrap; }
.app-data-table td { padding: 14px 16px; border-bottom: 1px solid var(--border); vertical-align: top; overflow-wrap: anywhere; }
.app-data-table tbody tr:last-child td { border-bottom: 0; }
.app-data-table tbody tr:hover { background: var(--surface-soft); }
.app-data-table__empty { min-height: 160px; }
</style>
