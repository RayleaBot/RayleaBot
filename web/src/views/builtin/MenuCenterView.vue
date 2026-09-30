<script setup lang="ts">
import AppTagsInput from '@/components/AppTagsInput.vue'
import AppTabs from '@/components/AppTabs.vue'
import PluginPicker from '@/components/plugins/PluginPicker.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppTag from '@/components/AppTag.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppButton from '@/components/AppButton.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { SaveIcon } from '@lucide/vue'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import NativeTemplatePreviewFrame from '@/components/templates/NativeTemplatePreviewFrame.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import {
  buildMenuTriggerExamples,
  buildPluginMenuGroups,
  buildRootMenuItems,
  defaultMenuCommands,
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
  .filter((plugin) => plugin.state === 'running')
  .sort((left, right) => compareLabel(left.name, right.name) || compareLabel(left.id, right.id)))

const selectedPlugin = computed(() => (
  pluginsStore.knownItems.find((plugin) => plugin.id === selectedPluginId.value && plugin.state === 'running')
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
  return {
    title: plugin.name || plugin.id,
    subtitle: plugin.help?.summary || plugin.commands[0]?.description || plugin.id,
    command_prefixes: effectiveMenuPrefixes.value,
    groups: buildPluginMenuGroups(plugin, previewContext.value),
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
      <div class="menu-center-float-panel">
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
          <AppTag v-if="hasUnsavedChanges" class="menu-center-unsaved-tag">
            {{ t('builtinFeatures.menuCenter.unsaved') }}
          </AppTag>
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

      <div class="menu-preview-area">
        <AppTabs v-model="activeTab" class="menu-center-tabs" keep-alive :items="[{ value: 'root', label: t('builtinFeatures.menuCenter.preview.rootTitle') }, { value: 'plugin', label: t('builtinFeatures.menuCenter.preview.pluginTitle') }]">
          <template #extra>
            <PluginPicker
              v-show="activeTab === 'plugin'"
              v-model="selectedPluginId"
              running-only
              align="end"
              :label="t('builtinFeatures.menuCenter.preview.selectedPlugin')"
              :placeholder="t('builtinFeatures.menuCenter.preview.allPlugins')"
              class="menu-center-plugin-select"

              data-testid="menu-center-plugin-select"
            />
          </template>

          <template #root>
            <div class="menu-preview-card">
              <p v-if="nextCursor" role="status">{{ t('builtinFeatures.menuCenter.preview.partial') }}</p>
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
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-sm);
  margin-top: var(--space-md);
  padding-top: var(--space-sm);
  border-top: 1px solid var(--border);
}

.menu-center-actions .app-button {
  margin-inline-start: auto;
}

.menu-center-layout {
  --menu-center-panel-width: 320px;
  --menu-center-panel-inset: var(--space-md);
  --menu-center-preview-max-width: 1040px;
  --menu-center-preview-top-space: 0px;

  display: grid;
  grid-template-columns: var(--menu-center-panel-width) minmax(0, 1fr);
  gap: var(--space-lg);
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

// The command fields and the preview sit directly on the menu center box.
.menu-center-float-panel {
  width: 100%;
  align-self: start;
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.menu-center-unsaved-tag {
  color: var(--text-attention);
  background: var(--surface-attention);
  border-color: var(--border-attention);
  font-size: 13px;
  margin: 0;
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

// The tabs and the full-height preview stage form one column centered in the space beside the fields.
.menu-preview-area {
  min-width: 0;
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 0;
  padding-top: var(--menu-center-preview-top-space);
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


.menu-preview-empty {
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 320px;
}
</style>
