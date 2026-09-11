<script setup lang="ts">
import { computed, inject, nextTick, ref, watch } from 'vue'
import { CheckIcon, ChevronDownIcon, SearchIcon } from '@lucide/vue'
import { motion } from 'motion-v'
import {
  ComboboxAnchor, ComboboxContent, ComboboxInput, ComboboxItem, ComboboxItemIndicator,
  ComboboxPortal, ComboboxRoot, ComboboxTrigger, ComboboxViewport, ComboboxVirtualizer,
} from 'reka-ui'
import { getTimeZonePickerChoices } from '@/lib/time-zone-picker'
import { DEFAULT_TIME_ZONE, formatTimeZoneOffset, isSupportedTimeZone, timeZoneOffsetMinutes } from '@/lib/time-zone'
import { overlayLayerKey } from './overlay-layer'
import { useOverlayMotion } from '@/motion/presets'

defineOptions({ inheritAttrs: false })
defineProps<{ disabled?: boolean; id?: string }>()
const model = defineModel<string>({ required: true })
const open = ref(false)
const active = ref(false)
const overlayMotion = useOverlayMotion()
const search = ref('')
const searchRef = ref<{ $el: HTMLInputElement } | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const layer = inject(overlayLayerKey, computed(() => 1100))
const at = ref(new Date())
const selectedID = computed(() => model.value.trim() || DEFAULT_TIME_ZONE)
const options = computed(() => getTimeZonePickerChoices(selectedID.value).map(zone => {
  const supported = isSupportedTimeZone(zone.id)
  return {
    ...zone, supported,
    offset: supported ? formatTimeZoneOffset(zone.id, at.value) : '暂不支持',
    minutes: supported ? timeZoneOffsetMinutes(zone.id, at.value) : Infinity,
    keywords: [zone.id, zone.city, zone.region, 'canonical' in zone ? zone.canonical : '', zone.id === 'Asia/Shanghai' ? '北京 中国 China Beijing UTC+8' : ''].join(' ').toLowerCase(),
  }
}).sort((a, b) => a.minutes - b.minutes || a.id.localeCompare(b.id)))
const selected = computed(() => options.value.find(zone => zone.id === selectedID.value))
const results = computed(() => {
  const normalized = search.value.toLowerCase().replaceAll('−', '-').replace(/utc([+-])(\d{1,2})(?::(\d{2}))?\b/g, (_, sign, hours, minutes) => `utc${sign}${hours.padStart(2, '0')}:${minutes || '00'}`)
  const terms = normalized.split(/\s+/).filter(Boolean)
  return options.value.filter(zone => terms.every(term => `${zone.keywords} ${zone.offset.replaceAll('−', '-')}`.toLowerCase().includes(term)))
})
const listHeight = computed(() => results.value.length ? Math.min(352, results.value.length * 44 + 8) : 92)

watch(open, async value => {
  if (value) {
    active.value = true
    search.value = ''
    at.value = new Date()
    await nextTick()
    if (open.value) searchRef.value?.$el?.focus()
  }
})

function motionComplete(definition: unknown) {
  if (!open.value && definition && typeof definition === 'object' && 'opacity' in definition && definition.opacity === 0) active.value = false
}

function restoreTriggerFocus() {
  void nextTick(() => triggerRef.value?.focus())
}

function select(value: unknown) {
  if (value && typeof value === 'object' && 'id' in value && typeof value.id === 'string') model.value = value.id
  restoreTriggerFocus()
}
</script>

<template>
  <ComboboxRoot :model-value="selected" v-model:open="open" by="id" :disabled="disabled" ignore-filter :reset-search-term-on-select="false" class="timezone-select" @update:model-value="select">
    <ComboboxAnchor as-child>
      <ComboboxTrigger as-child tabindex="0">
        <button :id="id" ref="triggerRef" type="button" class="timezone-select__trigger" :disabled="disabled" v-bind="$attrs" @keydown.down.prevent="open = true" @keydown.up.prevent="open = true">
          <span class="timezone-select__selection"><span class="timezone-select__offset">{{ selected?.offset }}</span><span>{{ selected?.city || selectedID }}</span></span>
          <motion.span class="timezone-select__chevron" :initial="false" :animate="{ rotate: open && overlayMotion.transition.duration ? 180 : 0 }" :transition="overlayMotion.transition" aria-hidden="true"><ChevronDownIcon :size="16" /></motion.span>
        </button>
      </ComboboxTrigger>
    </ComboboxAnchor>
    <ComboboxPortal>
      <ComboboxContent v-if="active" force-mount as-child position="popper" align="end" :side-offset="6" :collision-padding="12" @escape-key-down="restoreTriggerFocus">
        <motion.div class="timezone-select__content" :style="{ zIndex: layer + 5 }" :inert="!open" :initial="overlayMotion.initial" :animate="open ? overlayMotion.animate : overlayMotion.exit" :transition="overlayMotion.transition" @animation-complete="motionComplete">
          <div class="timezone-select__search"><SearchIcon :size="17" aria-hidden="true" /><ComboboxInput ref="searchRef" v-model="search" :display-value="() => search" placeholder="搜索城市、地区或 UTC 偏移" aria-label="搜索时区" /></div>
          <div class="timezone-select__summary"><span>地区时区 · {{ results.length }}</span><span>偏移按当前日期显示</span></div>
          <ComboboxViewport as-child>
            <motion.div class="timezone-select__viewport" :initial="false" :animate="{ height: listHeight }" :transition="overlayMotion.transition">
              <!-- Reka recycles by index and memoizes item attributes; a new query must also refresh IANA titles. -->
              <ComboboxVirtualizer :key="search" v-slot="{ option }" :options="results" :estimate-size="44" :text-content="zone => zone.keywords">
                <ComboboxItem :key="option.id" :value="option" :title="option.id" :disabled="!option.supported" class="timezone-select__option">
                  <span class="timezone-select__place">{{ option.city }}<small v-if="option.region && option.region !== option.city"> · {{ option.region }}</small><small v-if="option.retained" class="timezone-select__retained">当前配置</small></span>
                  <span class="timezone-select__offset">{{ option.offset }}</span>
                  <ComboboxItemIndicator class="timezone-select__check"><CheckIcon :size="16" /></ComboboxItemIndicator>
                </ComboboxItem>
              </ComboboxVirtualizer>
              <p v-if="!results.length" class="timezone-select__empty">没有匹配的时区，试试城市名或 UTC 偏移。</p>
            </motion.div>
          </ComboboxViewport>
        </motion.div>
      </ComboboxContent>
    </ComboboxPortal>
  </ComboboxRoot>
</template>

<style lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.timezone-select { width: 100%; min-width: 0; }
.timezone-select__trigger { display: flex; align-items: center; justify-content: space-between; gap: 10px; width: 100%; min-height: 44px; padding: 10px 12px; border: 1px solid var(--border-strong); border-radius: var(--radius-md); background: var(--surface-strong); color: var(--text); font: inherit; text-align: left; cursor: pointer; }
.timezone-select__trigger:focus-visible { outline: 1px solid var(--focus); outline-offset: -2px; }
.timezone-select__trigger:disabled { cursor: not-allowed; opacity: .5; }
.timezone-select__selection { display: flex; align-items: baseline; gap: 10px; min-width: 0; flex-wrap: wrap; }
.timezone-select__offset { flex: none; font-size: 12px; color: var(--muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
.timezone-select__chevron { display: flex; flex: none; color: var(--muted); }
.timezone-select__content { width: min(440px, calc(100vw - 24px)); overflow: hidden; transform-origin: var(--reka-combobox-content-transform-origin); border-radius: 12px; background: var(--surface-strong); color: var(--text); box-shadow: var(--shadow-floating); }
.timezone-select__search { display: flex; align-items: center; gap: 10px; margin: 8px; padding: 0 10px; min-height: 42px; border-bottom: 1px solid var(--border); color: var(--muted); }
.timezone-select__search input { flex: 1; min-width: 0; width: 100%; padding: 10px 0; border: 0; outline: none; background: transparent; color: var(--text); font-size: 14px; }
.timezone-select__search input::placeholder { color: var(--muted); }
.timezone-select__summary { display: flex; justify-content: space-between; gap: 12px; padding: 0 18px 8px; color: var(--muted); font-size: 12px; }
.timezone-select__viewport { max-height: max(0px, calc(var(--reka-combobox-content-available-height, 500px) - 90px)); overflow-y: auto; padding: 0 6px 6px; overscroll-behavior: contain; }
.timezone-select__viewport[data-reka-combobox-viewport] { scrollbar-width: thin; scrollbar-color: var(--border-strong) transparent; scrollbar-gutter: stable; }
.timezone-select__viewport[data-reka-combobox-viewport]:hover { scrollbar-color: var(--muted) transparent; }
@supports selector(::-webkit-scrollbar) {
  .timezone-select__viewport[data-reka-combobox-viewport],
  .timezone-select__viewport[data-reka-combobox-viewport]:hover { scrollbar-width: auto; scrollbar-color: auto; }
  .timezone-select__viewport[data-reka-combobox-viewport]::-webkit-scrollbar { display: block; width: 10px; }
  .timezone-select__viewport::-webkit-scrollbar-track { background: transparent; margin-block: 5px; }
  .timezone-select__viewport::-webkit-scrollbar-thumb { border: 2px solid transparent; border-radius: 999px; background: var(--border-strong); background-clip: padding-box; }
  .timezone-select__viewport::-webkit-scrollbar-thumb:hover { background-color: var(--muted); }
  .timezone-select__viewport::-webkit-scrollbar-thumb:active { background-color: var(--text); }
  .timezone-select__viewport::-webkit-scrollbar-button { display: none; width: 0; height: 0; }
}
.timezone-select__option { display: flex; align-items: center; gap: 10px; width: 100%; min-height: 44px; padding: 9px 30px 9px 12px; border-radius: 7px; position: relative; outline: none; cursor: pointer; }
.timezone-select__option[data-highlighted] { background: var(--surface-soft); }
.timezone-select__option[data-state=checked] { background: var(--brand-soft); }
.timezone-select__option[data-disabled] { opacity: .5; cursor: not-allowed; }
.timezone-select__place { flex: 1; min-width: 0; font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.timezone-select__place small { font-size: 12px; color: var(--muted); }
.timezone-select__retained { margin-left: 8px; }
.timezone-select__check { position: absolute; right: 10px; color: var(--brand-foreground); }
.timezone-select__empty { margin: 0; padding: 28px 18px; color: var(--muted); font-size: 14px; line-height: 1.6; }
@media (max-width: #{bp.$phone - 1px}) { .timezone-select__search input { font-size: 16px; } }
@media (forced-colors: active) { .timezone-select__content { border: 1px solid CanvasText; } .timezone-select__option[data-highlighted] { outline: 1px solid Highlight; } }
</style>
