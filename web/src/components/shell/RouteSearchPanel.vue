<script setup lang="ts">
import { nextTick, computed, ref, useTemplateRef, watch } from 'vue'
import { EnterOutlined } from '@ant-design/icons-vue'

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

const results = computed(() => {
  const normalizedKeyword = keyword.value.trim().toLowerCase()
  const sourceItems = props.items
    .filter((item) => item.title)
    .map((item) => ({
      ...item,
      score: getSearchScore(item, normalizedKeyword),
    }))
    .filter((item) => item.score > 0 || normalizedKeyword.length === 0)
    .sort((left, right) => {
      if (left.score === right.score) {
        return left.title.localeCompare(right.title, 'zh-CN')
      }
      return right.score - left.score
    })

  return sourceItems.slice(0, 12)
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
  <a-modal
    :open="open"
    :footer="null"
    :closable="false"
    :mask-closable="true"
    :width="640"
    centered
    class="route-search-modal"
    data-testid="route-search-modal"
    @cancel="close"
  >
    <div class="route-search-panel">
      <div class="route-search-panel__input">
        <a-input
          ref="inputRef"
          v-model:value="keyword"
          size="large"
          :placeholder="t('shell.searchPlaceholder')"
          @keydown="handleInputKeydown"
        />
      </div>

      <div v-if="results.length > 0" class="route-search-panel__results">
        <button
          v-for="(item, index) in results"
          :key="item.key"
          type="button"
          :class="['route-search-panel__result', { 'is-active': index === activeIndex }]"
          @mouseenter="activeIndex = index"
          @click="selectResult(item.path)"
        >
          <div class="route-search-panel__meta">
            <strong>{{ item.title }}</strong>
            <span>{{ item.path }}</span>
          </div>
          <EnterOutlined />
        </button>
      </div>

      <a-empty v-else :description="t('shell.searchEmpty')" />

      <div class="route-search-panel__footer">
        <span>{{ t('shell.searchShortcutHint') }}</span>
      </div>
    </div>
  </a-modal>
</template>

<style scoped lang="scss">
.route-search-panel {
  display: grid;
  gap: 16px;
}

.route-search-panel__results {
  display: grid;
  gap: 8px;
  max-height: 420px;
  overflow-y: auto;
}

.route-search-panel__result {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.2s ease, background-color 0.2s ease;

  &:hover,
  &.is-active {
    border-color: color-mix(in srgb, var(--accent) 36%, transparent);
    background: color-mix(in srgb, var(--accent) 8%, transparent);
  }
}

.route-search-panel__meta {
  display: grid;
  gap: 4px;

  strong {
    font-size: 0.96rem;
  }

  span {
    color: var(--muted);
    font-size: 0.82rem;
  }
}

.route-search-panel__footer {
  color: var(--muted);
  font-size: 0.8rem;
}
</style>
