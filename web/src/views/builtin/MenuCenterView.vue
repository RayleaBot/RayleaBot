<script setup lang="ts">
import AppTagsInput from '@/components/AppTagsInput.vue'
import AppTabs from '@/components/AppTabs.vue'
import PluginPicker from '@/components/plugins/PluginPicker.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppTag from '@/components/AppTag.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppButton from '@/components/AppButton.vue'
import { computed, onMounted, ref, useId, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ChevronDownIcon, SaveIcon, SlidersHorizontalIcon } from '@lucide/vue'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import NativeTemplatePreviewFrame from '@/components/templates/NativeTemplatePreviewFrame.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import {
  buildMenuTriggerExamples,
  buildPluginMenuGroups,
  buildPrefixChips,
  buildRootMenuItems,
  defaultMenuCommands,
  isMenuPreviewPlugin,
  normalizeMenuTokens,
  renderMenuPreviewFooter,
} from '@/lib/menu-preview'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import { usePluginCollection } from '@/lib/use-plugin-collection'
import type { ConfigDocument } from '@/types/api'

const configStore = useConfigStore()
const pluginsStore = usePluginsStore()
const { document: configDocument, error: configError, loading: configLoading, saving } = storeToRefs(configStore)
const pluginCollection = usePluginCollection()
const { error: pluginsError, loading: pluginsLoading, items: sortedItems, total, nextCursor, loadingMore } = pluginCollection

const draftCommands = ref<string[]>([])
const draftPrefixes = ref<string[]>([])
const selectedPluginId = ref<string>('')
const activeTab = ref<'root' | 'plugin'>('root')
const settingsCollapsed = ref(false)
const settingsId = useId()

const pageError = computed(() => configError.value ?? pluginsError.value)
const loading = computed(() => configLoading.value || pluginsLoading.value)

useToastFeedback(computed(() => (
  pageError.value && configDocument.value
    ? {
        key: `menu-center-error:${pageError.value}`,
        level: 'error' as const,
        message: pageError.value,
      }
    : null
)))

const inheritedCommandPrefixes = computed(() => {
  const prefixes = normalizeMenuTokens(configDocument.value?.command?.prefixes)
  return prefixes.length > 0 ? prefixes : ['/']
})
const inheritedPrefixLabel = computed(() => inheritedCommandPrefixes.value.join('、'))
const effectiveMenuPrefixes = computed(() => draftPrefixes.value.length > 0 ? draftPrefixes.value : inheritedCommandPrefixes.value)
const previewContext = computed(() => ({
  prefixes: effectiveMenuPrefixes.value,
  defaultPermission: configDocument.value?.permission?.default_level,
}))
const footerTemplate = computed(() => configDocument.value?.render?.footer_template)

const enabledPlugins = computed(() => sortedItems.value
  .filter(isMenuPreviewPlugin)
  .sort((left, right) => compareLabel(left.name, right.name) || compareLabel(left.id, right.id)))

const selectedPlugin = computed(() => (
  pluginsStore.knownItems.find((plugin) => plugin.id === selectedPluginId.value && isMenuPreviewPlugin(plugin))
    ?? enabledPlugins.value.find((plugin) => plugin.id === selectedPluginId.value)
    ?? null
))

const rootMenuTriggerExamples = computed(() => {
  const plugin = enabledPlugins.value[0]
  return buildMenuTriggerExamples(plugin ? plugin.name || plugin.id : null, effectiveMenuPrefixes.value, draftCommands.value)
})

const rootPreviewData = computed(() => ({
  title: '插件菜单',
  subtitle: '当前可用插件',
  command_prefixes: effectiveMenuPrefixes.value,
  trigger_examples: rootMenuTriggerExamples.value,
  items: buildRootMenuItems(enabledPlugins.value),
  render_footer: renderMenuPreviewFooter(footerTemplate.value),
}))

const selectedPluginPreviewData = computed(() => {
  const plugin = selectedPlugin.value
  if (!plugin) {
    return {
      title: '插件菜单',
      subtitle: '当前没有可预览的插件菜单。',
      groups: [],
      render_footer: renderMenuPreviewFooter(footerTemplate.value),
    }
  }
  // A plugin's own menu page shows and uses the prefixes that address that plugin, not the menu's.
  return {
    title: plugin.name || plugin.id,
    subtitle: plugin.description?.trim() ?? '',
    command_prefixes: plugin.command_prefixes,
    prefix_chips: buildPrefixChips(plugin),
    groups: buildPluginMenuGroups(plugin, { ...previewContext.value, prefixes: plugin.command_prefixes }),
    render_footer: renderMenuPreviewFooter(footerTemplate.value, plugin),
  }
})

const hasUnsavedChanges = computed(() => {
  const source = configDocument.value
  if (!source) {
    return false
  }
  return JSON.stringify(draftCommands.value) !== JSON.stringify(normalizeMenuTokens(source.builtin_features?.menu?.commands, defaultMenuCommands))
    || JSON.stringify(draftPrefixes.value) !== JSON.stringify(normalizeMenuTokens(source.builtin_features?.menu?.prefixes))
})

watch(configDocument, (value, previous) => {
  if (value && previous && JSON.stringify(value) === JSON.stringify(previous)) return
  draftCommands.value = normalizeMenuTokens(value?.builtin_features?.menu?.commands, defaultMenuCommands)
  draftPrefixes.value = normalizeMenuTokens(value?.builtin_features?.menu?.prefixes)
}, { immediate: true })

watch(enabledPlugins, (plugins) => {
  if (!selectedPluginId.value && plugins.length) selectedPluginId.value = plugins[0].id
}, { immediate: true })

onMounted(() => {
  void loadPage()
})

async function loadPage() {
  await Promise.allSettled([
    configStore.fetchConfig(),
    pluginCollection.load({}),
  ])
}

function compareLabel(left: string, right: string) {
  return left.localeCompare(right, 'zh-CN')
}

function patchBuiltinMenuConfig(source: ConfigDocument) {
  return {
    ...source,
    builtin_features: {
      ...(source.builtin_features ?? {}),
      menu: {
        commands: normalizeMenuTokens(draftCommands.value, defaultMenuCommands),
        prefixes: normalizeMenuTokens(draftPrefixes.value),
      },
    },
  } as ConfigDocument
}

async function save() {
  if (!configDocument.value || !hasUnsavedChanges.value) {
    return
  }
  await configStore.saveConfig(patchBuiltinMenuConfig(configDocument.value))
  notifySuccess(t('builtinFeatures.menuCenter.saved'))
}
</script>

<template>
  <AppPage :title="t('builtinFeatures.menuCenter.title')" full-height>
    <RetryPanel
      v-if="pageError && !configDocument"
      :title="t('errors.common.loadFailed')"
      :description="pageError"
      :loading="loading"
      @retry="loadPage"
    />

    <div v-else class="menu-center-layout">
      <section class="menu-center-float-panel" :aria-labelledby="`${settingsId}-title`" data-testid="menu-center-settings">
        <h2 class="menu-center-float-panel__heading">
          <button
            type="button"
            class="menu-center-float-panel__toggle"
            :aria-expanded="!settingsCollapsed"
            :aria-controls="`${settingsId}-body`"
            data-testid="menu-center-settings-toggle"
            @click="settingsCollapsed = !settingsCollapsed"
          >
            <SlidersHorizontalIcon class="menu-center-float-panel__icon" aria-hidden="true" />
            <span :id="`${settingsId}-title`">{{ t('builtinFeatures.menuCenter.settings') }}</span>
            <AppTag v-if="hasUnsavedChanges" class="menu-center-unsaved-tag">
              {{ t('builtinFeatures.menuCenter.unsaved') }}
            </AppTag>
            <ChevronDownIcon class="menu-center-float-panel__chevron" aria-hidden="true" />
          </button>
        </h2>

        <Transition name="menu-center-settings">
          <div v-show="!settingsCollapsed" :id="`${settingsId}-body`" class="menu-center-float-panel__collapse">
            <div class="menu-center-float-panel__inner">
              <div class="menu-center-float-panel__content">
                <div class="menu-center-float-panel__body">
                  <div class="menu-center-float-panel__field">
                    <label class="menu-center-float-panel__label">{{ t('builtinFeatures.menuCenter.commands.label') }}</label>
                    <AppTagsInput
                      v-model="draftCommands" :aria-label="t('builtinFeatures.menuCenter.commands.label')"

                      :separators="[',', '，', ' ']"
                      :placeholder="t('builtinFeatures.menuCenter.commands.placeholder')"
                      data-testid="menu-center-commands"
                      class="menu-center-float-panel__select"
                    />
                  </div>

                  <div class="menu-center-float-panel__field">
                    <label class="menu-center-float-panel__label">{{ t('builtinFeatures.menuCenter.prefixes.label') }}</label>
                    <AppTagsInput
                      v-model="draftPrefixes" :aria-label="t('builtinFeatures.menuCenter.prefixes.label')"

                      :separators="[',', '，', ' ']"
                      :placeholder="t('builtinFeatures.menuCenter.prefixes.placeholder')"
                      data-testid="menu-center-prefixes"
                      class="menu-center-float-panel__select"
                    />
                    <div v-if="draftPrefixes.length === 0" class="menu-center-field-note" data-testid="menu-center-inherited-prefixes">
                      {{ t('builtinFeatures.menuCenter.prefixes.inherited', { prefixes: inheritedPrefixLabel }) }}
                    </div>
                  </div>
                </div>
                <div class="menu-center-actions">
                  <AppButton
                    variant="default"
                    :disabled="!hasUnsavedChanges"
                    :loading="saving"
                    data-testid="menu-center-save"
                    @click="save"
                  >
                    <template #icon><SaveIcon /></template>
                    {{ t('builtinFeatures.menuCenter.save') }}
                  </AppButton>
                </div>
              </div>
            </div>
          </div>
        </Transition>
      </section>

      <div class="menu-preview-area">
        <AppTabs v-model="activeTab" class="menu-center-tabs" keep-alive :items="[{ value: 'root', label: t('builtinFeatures.menuCenter.preview.rootTitle') }, { value: 'plugin', label: t('builtinFeatures.menuCenter.preview.pluginTitle') }]">
          <template #extra>
            <PluginPicker
              v-show="activeTab === 'plugin'"
              v-model="selectedPluginId"
              running-only
              align="end"
              :label="t('builtinFeatures.menuCenter.preview.selectedPlugin')"
              :placeholder="t('plugins.picker.title')"
              class="menu-center-plugin-select"

              data-testid="menu-center-plugin-select"
            />
          </template>

          <template #root>
            <div class="menu-preview-card">
              <p v-if="nextCursor" class="menu-preview-partial" role="status">{{ t('builtinFeatures.menuCenter.preview.partial') }}</p>
              <AppCollectionPagination :loaded="sortedItems.length" :total="total" :next-cursor="nextCursor" :loading="pluginsLoading || loadingMore" @more="pluginCollection.loadMore().catch(() => undefined)" />
              <NativeTemplatePreviewFrame
                template-id="help.menu"
                :data="rootPreviewData"
                data-testid="menu-center-root-preview"
              />
            </div>
          </template>

          <template #plugin>
            <div class="menu-preview-card">
              <template v-if="selectedPlugin">
                <NativeTemplatePreviewFrame
                  template-id="help.menu"
                  :data="selectedPluginPreviewData"
                  data-testid="menu-center-plugin-preview"
                />
              </template>
              <AppEmptyState
                v-else
                :description="t('builtinFeatures.menuCenter.preview.noPlugins')"
                class="menu-preview-empty"
              />
            </div>
          </template>
        </AppTabs>
      </div>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
.menu-center-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--space-md);
  padding-top: var(--space-sm);
  border-top: 1px solid var(--border);
}

.menu-center-layout {
  --menu-center-panel-width: 352px;
  --menu-center-preview-max-width: 1040px;
  // The settings window lines up with the top of the stage: below the AppTabs row (44px triggers over a 1px rule)
  // and the gap under it.
  --menu-center-stage-top: calc(var(--space-lg) + 45px + var(--space-md));

  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1 1 auto;
  padding: var(--space-lg);
  border: 1px solid transparent;
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
  --control-fill: var(--surface-raised);
  --control-fill-hover: color-mix(in srgb, var(--surface-raised) 97%, var(--text));
}

// The settings float over the box like the other floating surfaces, so the preview can take the middle of the whole
// box. On narrow windows the open window may cover the stage's left edge; its title bar rolls it up.
.menu-center-float-panel {
  position: absolute;
  z-index: 1;
  top: var(--menu-center-stage-top);
  left: var(--space-lg);
  width: var(--menu-center-panel-width);
  padding: 4px;
  border: 1px solid transparent;
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-floating);
}

.menu-center-float-panel__heading {
  margin: 0;
  font: inherit;
}

.menu-center-float-panel__toggle {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  width: 100%;
  min-height: 40px;
  padding: 0 10px 0 12px;
  border: 0;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
  line-height: 1.4;
  text-align: start;
  cursor: pointer;
  transition: background-color var(--motion-fast) var(--motion-easing);
}

.menu-center-float-panel__toggle:hover {
  background: var(--nav-hover);
}

.menu-center-float-panel__toggle:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: -2px;
}

.menu-center-float-panel__icon {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  color: var(--muted);
}

.menu-center-float-panel__chevron {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  margin-inline-start: auto;
  color: var(--muted);
  transition: transform var(--motion-content) var(--motion-easing);
}

.menu-center-float-panel__toggle[aria-expanded="true"] .menu-center-float-panel__chevron {
  transform: rotate(180deg);
}

.menu-center-unsaved-tag {
  color: var(--text-attention);
  background: var(--surface-attention);
  border-color: var(--border-attention);
  font-size: 13px;
  font-weight: 400;
  margin: 0;
}

// The row that rolls up has no size of its own, or the track would stay open by that much and then snap shut;
// the padding lives on the content inside it.
.menu-center-float-panel__collapse {
  display: grid;
  grid-template-rows: 1fr;
}

.menu-center-float-panel__inner {
  display: flow-root;
  min-height: 0;
}

.menu-center-float-panel__content {
  padding: var(--space-sm) 12px 12px;
}

.menu-center-settings-enter-active,
.menu-center-settings-leave-active {
  overflow: hidden;
}

.menu-center-settings-enter-active {
  transition: grid-template-rows 260ms var(--motion-easing), opacity 200ms var(--motion-easing);
}

.menu-center-settings-leave-active {
  transition: grid-template-rows 200ms cubic-bezier(0.4, 0, 0.2, 1), opacity 120ms cubic-bezier(0.4, 0, 1, 1);
}

.menu-center-settings-enter-from,
.menu-center-settings-leave-to {
  grid-template-rows: 0fr;
  opacity: 0;
}

.menu-center-float-panel__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.menu-center-float-panel__field {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.menu-center-float-panel__label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  line-height: 1.4;
}

.menu-center-float-panel__select {
  width: 100%;
}

.menu-center-field-note {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
}

// The tabs and the full-height preview stage form one column centered in the whole box.
.menu-preview-area {
  min-width: 0;
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 0;
}

.menu-center-tabs {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  width: min(100%, var(--menu-center-preview-max-width));
  min-height: 0;

  :deep(.app-tabs__content) {
    flex: 1 1 auto;
    min-height: 0;
  }

  :deep(.app-tabs__content) {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  :deep(.app-tabs__header) {
    margin-bottom: var(--space-md);
  }

  :deep(.app-tabs__trigger) {
    font-weight: 500;
  }

  :deep(.app-tabs__extra) {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }
}

// Sized to the chosen plugin's name and ID on one line and anchored at the right end of the tab row,
// so both tabs start the stage at the same height; only a very long name ends in an ellipsis.
.menu-center-plugin-select {
  width: auto;
  max-width: 480px;
}

.menu-preview-card {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  width: min(100%, var(--menu-center-preview-max-width));
  min-width: 0;
  min-height: 0;
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
}

// The settings window may cover the stage's left edge, so the partial-preview notice and paging sit in the middle.
.menu-preview-partial {
  text-align: center;
}

.menu-preview-card :deep(.collection-pagination) {
  justify-content: center;
}

.menu-preview-empty {
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 320px;
}

@media (prefers-reduced-motion: reduce) {
  .menu-center-float-panel__toggle,
  .menu-center-float-panel__chevron,
  .menu-center-settings-enter-active,
  .menu-center-settings-leave-active {
    transition: none;
  }
}

@media (forced-colors: active) {
  .menu-center-float-panel {
    border-color: CanvasText;
    box-shadow: none;
  }
}
</style>
