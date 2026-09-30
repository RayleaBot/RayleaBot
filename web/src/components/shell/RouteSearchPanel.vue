<script setup lang="ts">
import { nextTick, computed, ref, useId, useTemplateRef, watch } from 'vue'
import { CornerDownLeftIcon as EnterOutlined } from '@lucide/vue'
import AppDialog from '@/components/AppDialog.vue'
import AppInput from '@/components/AppInput.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'

import type { AppNavigationItem } from '@/access/menu'
import { t } from '@/i18n'

const props = defineProps<{
  items: AppNavigationItem[]
  open: boolean
}>()

const emit = defineEmits<{
  'navigate': [path: string]
  'update:open': [value: boolean]
}>()

const keyword = ref('')
const activeIndex = ref(0)
const inputRef = useTemplateRef('inputRef')
const listboxId = useId()

// Without a keyword every page is listed in sidebar order; matches keep that order within the same score.
const results = computed(() => {
  const normalizedKeyword = keyword.value.trim().toLowerCase()
  return props.items
    .filter((item) => item.title)
    .map((item, order) => ({
      ...item,
      order,
      score: getSearchScore(item, normalizedKeyword),
    }))
    .filter((item) => item.score > 0 || normalizedKeyword.length === 0)
    .sort((left, right) => right.score - left.score || left.order - right.order)
})

function optionId(index: number) {
  return `${listboxId}-option-${index}`
}

watch(activeIndex, async (index) => {
  await nextTick()
  document.getElementById(optionId(index))?.scrollIntoView({ block: 'nearest' })
})

watch(
  () => props.open,
  async (open) => {
    if (!open) {
      keyword.value = ''
      activeIndex.value = 0
      return
    }

    await nextTick()
    inputRef.value?.focus()
  },
)

watch(results, (items) => {
  if (items.length === 0) {
    activeIndex.value = 0
    return
  }

  if (activeIndex.value > items.length - 1) {
    activeIndex.value = 0
  }
})

function close() {
  emit('update:open', false)
}

function selectResult(path: string) {
  emit('navigate', path)
  close()
}

function moveSelection(step: 1 | -1) {
  if (results.value.length === 0) {
    return
  }

  const nextIndex = activeIndex.value + step
  if (nextIndex < 0) {
    activeIndex.value = results.value.length - 1
    return
  }

  activeIndex.value = nextIndex % results.value.length
}

function submitSelection() {
  const target = results.value[activeIndex.value]
  if (!target) {
    return
  }

  selectResult(target.path)
}

function handleInputKeydown(event: KeyboardEvent) {
  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      moveSelection(1)
      return
    case 'ArrowUp':
      event.preventDefault()
      moveSelection(-1)
      return
    case 'Enter':
      event.preventDefault()
      submitSelection()
      return
    case 'Escape':
      event.preventDefault()
      close()
      return
    default:
      return
  }
}

function getSearchScore(item: AppNavigationItem, normalizedKeyword: string) {
  if (!normalizedKeyword) {
    return 1
  }

  const title = item.title.toLowerCase()
  const path = item.path.toLowerCase()

  if (title === normalizedKeyword) {
    return 5
  }

  if (title.startsWith(normalizedKeyword)) {
    return 4
  }

  if (title.includes(normalizedKeyword)) {
    return 3
  }

  if (path.includes(normalizedKeyword)) {
    return 2
  }

  return 0
}
</script>

<template>
  <AppDialog :open="open" :title="t('shell.search')" :width="640" initial-focus="#route-search-input" class="route-search-modal" data-testid="route-search-modal" @close="close">
    <div class="route-search-panel">
      <div class="route-search-panel__input">
        <AppInput
          ref="inputRef"
          v-model="keyword"
          id="route-search-input"
          role="combobox"
          aria-autocomplete="list"
          :aria-expanded="results.length > 0"
          :aria-controls="listboxId"
          :aria-activedescendant="results.length > 0 ? optionId(activeIndex) : undefined"
          :aria-label="t('shell.searchPlaceholder')"
          :placeholder="t('shell.searchPlaceholder')"
          @keydown="handleInputKeydown"
        />
      </div>

      <div v-if="results.length > 0" :id="listboxId" class="route-search-panel__results" role="listbox" :aria-label="t('shell.search')">
        <button
          v-for="(item, index) in results"
          :id="optionId(index)"
          :key="item.key"
          type="button"
          role="option"
          tabindex="-1"
          :aria-selected="index === activeIndex"
          :class="['route-search-panel__result', { 'is-active': index === activeIndex }]"
          @mouseenter="activeIndex = index"
          @click="selectResult(item.path)"
        >
          <div class="route-search-panel__meta">
            <strong>{{ item.title }}</strong>
            <span>{{ item.path }}</span>
          </div>
          <EnterOutlined v-if="index === activeIndex" class="route-search-panel__enter" aria-hidden="true" />
        </button>
      </div>

      <AppEmptyState v-else :description="t('shell.searchEmpty')" />

      <div class="route-search-panel__footer">
        <span>{{ t('shell.searchShortcutHint') }}</span>
      </div>
    </div>
  </AppDialog>
</template>

<style scoped lang="scss">
// The panel hangs from a fixed top line so filtering, which changes its height, never moves the input.
:global(.app-dialog.route-search-modal) {
  top: 14vh;
  translate: -50% 0;
  transform-origin: 50% 0;
}

.route-search-panel {
  display: grid;
  gap: 16px;
}

.route-search-panel__results {
  display: grid;
  gap: 2px;
  max-height: min(440px, 52vh);
  padding: 2px;
  overflow-y: auto;
}

.route-search-panel__result {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
  padding: 10px 14px;
  border: 1px solid transparent;
  border-radius: var(--radius-lg);
  background: transparent;
  color: var(--text);
  cursor: pointer;
  text-align: left;
  transition: background-color var(--motion-fast) var(--motion-easing), box-shadow var(--motion-fast) var(--motion-easing);

  &:hover,
  &.is-active {
    background: var(--surface-raised);
    box-shadow: var(--shadow-xs);
  }
}

.route-search-panel__enter {
  flex: none;
  width: 16px;
  height: 16px;
  color: var(--muted);
}

.route-search-panel__meta {
  display: grid;
  gap: 4px;

  strong {
    font-size: 14px;
  }

  span {
    color: var(--muted);
    font-size: 13px;
  }
}

.route-search-panel__footer {
  color: var(--muted);
  font-size: 13px;
}

</style>
