<script setup lang="ts">
import { computed, onDeactivated, ref, watch } from 'vue'
import { ChevronDownIcon } from '@lucide/vue'
import AppButton from '@/components/AppButton.vue'
import AppInput from '@/components/AppInput.vue'
import AppCheckbox from '@/components/AppCheckbox.vue'
import AppPopover from '@/components/AppPopover.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import { useFieldContext } from '@/components/form-context'
import { usePluginCollection } from '@/lib/use-plugin-collection'
import { usePluginsStore } from '@/stores/plugins'
import { t } from '@/i18n'
import type { PluginSummary } from '@/types/api'

const props = defineProps<{ multiple?: boolean; placeholder?: string; runningOnly?: boolean; label?: string }>()
const selection = defineModel<string | string[]>({ required: true })
const field = useFieldContext()
const plugins = usePluginsStore()
const collection = usePluginCollection()
const { items, total, nextCursor, loading, loadingMore, error } = collection
const open = ref(false)
const query = ref('')
const selectedIds = computed(() => (Array.isArray(selection.value) ? selection.value : [selection.value]).filter(Boolean))
const labels = computed(() => selectedIds.value.map(id => plugins.getPluginLabel(id)).join('、'))
const options = computed(() => items.value.filter(plugin => !props.runningOnly || plugin.state === 'running'))
const selectedOutsidePage = computed(() => selectedIds.value.filter(id => !options.value.some(plugin => plugin.id === id)))

watch(selectedIds, ids => {
  for (const id of ids) {
    if (plugins.getPluginDisplayName(id) === id && !plugins.detailErrorsByPluginId[id]) void plugins.ensureDetail(id).catch(() => undefined)
  }
}, { immediate: true })

watch([query, open], ([value, visible], previous, cleanup) => {
  collection.cancel()
  if (!visible) return
  const load = () => { void collection.load({ query: value }).catch(() => undefined) }
  if (!previous[1]) { load(); return }
  const timer = setTimeout(load, 250)
  cleanup(() => clearTimeout(timer))
})
onDeactivated(() => { open.value = false; collection.cancel() })

function choose(id: string) {
  if (props.multiple) selection.value = selectedIds.value.includes(id) ? selectedIds.value.filter(value => value !== id) : [...selectedIds.value, id]
  else { selection.value = id; open.value = false }
}
function optionLabel(plugin: PluginSummary) { return plugins.getPluginLabel(plugin.id, plugin.name) }
</script>

<template>
  <div class="plugin-picker">
  <AppPopover v-model:open="open" :label="label || t('plugins.picker.title')" :width="420">
    <AppButton :id="field?.id" class="plugin-picker__trigger" :aria-expanded="open" :aria-label="label">
      <span>{{ labels || placeholder || t('plugins.picker.title') }}</span><ChevronDownIcon :size="16" />
    </AppButton>
    <template #content>
      <AppInput v-model="query" allow-clear :aria-label="t('plugins.picker.search')" :placeholder="t('plugins.picker.search')" />
      <div class="plugin-picker__options" :aria-busy="loading">
        <template v-if="multiple">
          <AppCheckbox v-for="id in selectedOutsidePage" :key="id" :model-value="true" @update:model-value="choose(id)">{{ plugins.getPluginLabel(id) }}</AppCheckbox>
          <AppCheckbox v-for="plugin in options" :key="plugin.id" :model-value="selectedIds.includes(plugin.id)" @update:model-value="choose(plugin.id)">{{ optionLabel(plugin) }}</AppCheckbox>
        </template>
        <template v-else>
          <AppButton v-for="plugin in options" :key="plugin.id" variant="ghost" class="plugin-picker__option" :aria-pressed="selectedIds.includes(plugin.id)" @click="choose(plugin.id)">{{ optionLabel(plugin) }}</AppButton>
        </template>
        <span v-if="!loading && !options.length" role="status">{{ t('plugins.picker.empty') }}</span>
      </div>
      <div v-if="error" role="alert">{{ error }} <AppButton variant="link" @click="collection.load({ query }).catch(() => undefined)">{{ t('plugins.navigation.retry') }}</AppButton></div>
      <AppCollectionPagination :loaded="items.length" :total="total" :next-cursor="nextCursor" :loading="loading || loadingMore" @more="collection.loadMore().catch(() => undefined)" />
      <AppButton v-if="multiple && selectedIds.length" variant="link" @click="selection = []">{{ t('plugins.picker.clear') }}</AppButton>
    </template>
  </AppPopover>
  </div>
</template>

<style scoped>
.plugin-picker__trigger { justify-content: space-between; width: 100%; min-height: 40px; text-align: start; white-space: normal; }
.plugin-picker__trigger span { overflow-wrap: anywhere; }
.plugin-picker__options { display: grid; gap: 4px; max-height: 300px; overflow-y: auto; }
.plugin-picker__option { justify-content: flex-start; white-space: normal; text-align: start; }
</style>
