<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ReloadOutlined, SaveOutlined } from '@ant-design/icons-vue'

import { notifySuccess } from '@/adapter/feedback'
import NativeTemplatePreviewFrame from '@/components/NativeTemplatePreviewFrame.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getPrimaryCommandPrefix } from '@/lib/command-usage'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import type { ConfigDocument, PluginSummary } from '@/types/api'

const defaultMenuCommands = ['help', '帮助']
const defaultRenderFooterTemplate = 'Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}'
const previewDevelopmentVersion = '开发版本'
const previewNativeMenuPluginName = 'RayleaBot'

const configStore = useConfigStore()
const pluginsStore = usePluginsStore()
const { document: configDocument, error: configError, loading: configLoading, saving } = storeToRefs(configStore)
const { error: pluginsError, loading: pluginsLoading, sortedItems } = storeToRefs(pluginsStore)

const draftCommands = ref<string[]>([])
const draftPrefixes = ref<string[]>([])
const selectedPluginId = ref<string>('')
const activeTab = ref<'root' | 'plugin'>('root')

const pageError = computed(() => configError.value ?? pluginsError.value)
const loading = computed(() => configLoading.value || pluginsLoading.value)
const inheritedCommandPrefixes = computed(() => normalizeTokens(configDocument.value?.command?.prefixes).length > 0
  ? normalizeTokens(configDocument.value?.command?.prefixes)
  : ['/'])
const effectiveMenuPrefixes = computed(() => draftPrefixes.value.length > 0 ? draftPrefixes.value : inheritedCommandPrefixes.value)
const primaryMenuPrefix = computed(() => getPrimaryCommandPrefix(effectiveMenuPrefixes.value))

const enabledPlugins = computed(() => sortedItems.value
  .filter((plugin) => plugin.registration_state === 'installed' && plugin.desired_state === 'enabled')
  .sort((left, right) => compareLabel(left.name, right.name) || compareLabel(left.id, right.id)))

const selectedPlugin = computed(() => (
  enabledPlugins.value.find((plugin) => plugin.id === selectedPluginId.value)
    ?? enabledPlugins.value[0]
    ?? null
))

const pluginOptions = computed(() => enabledPlugins.value.map((plugin) => ({
  label: `${plugin.name}（${plugin.id}）`,
  value: plugin.id,
})))

const rootPreviewItems = computed(() => enabledPlugins.value
  .map((plugin) => ({
    name: plugin.name || plugin.id,
    description: plugin.help?.summary || plugin.commands[0]?.description || plugin.id,
  })))

const selectedPluginPreviewGroups = computed(() => {
  if (!selectedPlugin.value) {
    return []
  }

  const commandItems = selectedPlugin.value.commands.map((command) => ({
    name: command.name,
    command_prefixes: effectiveMenuPrefixes.value,
    description: command.description || command.name,
    permission: command.permission || 'everyone',
  }))
  const groups = commandItems.length > 0
    ? [{ title: '命令', items: commandItems }]
    : []

  for (const group of selectedPlugin.value.help?.groups ?? []) {
    const items = group.items.map((item) => ({
      name: item.command || item.title,
      command_prefixes: item.command ? effectiveMenuPrefixes.value : [],
      description: item.description || item.title,
      permission: item.permission || 'everyone',
    }))
    if (items.length > 0) {
      groups.push({ title: group.title, items })
    }
  }

  return groups
})

const rootPreviewData = computed(() => ({
  title: '插件菜单',
  subtitle: '当前可用插件',
  command_prefixes: effectiveMenuPrefixes.value,
  trigger_examples: rootMenuTriggerExamples.value,
  items: rootPreviewItems.value,
  render_footer: nativeMenuPreviewFooter.value,
}))

const selectedPluginPreviewData = computed(() => {
  const plugin = selectedPlugin.value
  if (!plugin) {
    return {
      title: '插件菜单',
      subtitle: '当前没有可预览的插件菜单。',
      groups: [],
      render_footer: nativeMenuPreviewFooter.value,
    }
  }
  return {
    title: plugin.name || plugin.id,
    subtitle: plugin.help?.summary || plugin.commands[0]?.description || plugin.id,
    command_prefixes: effectiveMenuPrefixes.value,
    groups: selectedPluginPreviewGroups.value,
    render_footer: nativeMenuPreviewFooter.value,
  }
})

const inheritedPrefixLabel = computed(() => inheritedCommandPrefixes.value.join('、'))
const nativeMenuPreviewFooter = computed(() => renderNativeMenuPreviewFooter(configDocument.value?.render?.footer_template))
const rootMenuTriggerExamples = computed(() => buildMenuTriggerExamples(enabledPlugins.value[0] ?? null))

const hasUnsavedChanges = computed(() => {
  const source = configDocument.value
  if (!source) {
    return false
  }
  return JSON.stringify(draftCommands.value) !== JSON.stringify(normalizeTokens(source.builtin_features?.menu?.commands, defaultMenuCommands))
    || JSON.stringify(draftPrefixes.value) !== JSON.stringify(normalizeTokens(source.builtin_features?.menu?.prefixes))
})

watch(configDocument, (value) => {
  draftCommands.value = normalizeTokens(value?.builtin_features?.menu?.commands, defaultMenuCommands)
  draftPrefixes.value = normalizeTokens(value?.builtin_features?.menu?.prefixes)
}, { immediate: true })

watch(enabledPlugins, (plugins) => {
  if (!plugins.length) {
    selectedPluginId.value = ''
    return
  }
  if (!plugins.some((plugin) => plugin.id === selectedPluginId.value)) {
    selectedPluginId.value = plugins[0].id
  }
}, { immediate: true })

onMounted(() => {
  void loadPage()
})

async function loadPage() {
  await Promise.allSettled([
    configStore.fetchConfig(),
    pluginsStore.fetchList(),
  ])
}

function normalizeTokens(values?: readonly string[] | null, fallback: string[] = []) {
  const seen = new Set<string>()
  const items: string[] = []
  for (const value of values ?? fallback) {
    const trimmed = String(value).trim()
    if (!trimmed || seen.has(trimmed)) {
      continue
    }
    seen.add(trimmed)
    items.push(trimmed)
  }
  return items
}

function compareLabel(left: string, right: string) {
  return left.localeCompare(right, 'zh-CN')
}

function secondaryMenuPrefix() {
  return effectiveMenuPrefixes.value[1] || primaryMenuPrefix.value
}

function buildMenuTriggerExamples(plugin: PluginSummary | null) {
  if (!plugin) {
    return []
  }
  const target = plugin.name || plugin.id
  const commands = normalizeTokens(draftCommands.value, defaultMenuCommands)
  const examples = [`${primaryMenuPrefix.value}${commands[0] || defaultMenuCommands[0]} ${target}`]
  if ((commands[1] || commands[0] || defaultMenuCommands[1])) {
    examples.push(`${secondaryMenuPrefix()}${target}${commands[1] || commands[0] || defaultMenuCommands[1]}`)
  }
  return examples
}

function renderNativeMenuPreviewFooter(template?: string) {
  const source = template?.trim() || defaultRenderFooterTemplate
  return source
    .replaceAll('{{rayleabot_version}}', previewDevelopmentVersion)
    .replaceAll('{{plugin_name}}', previewNativeMenuPluginName)
    .replaceAll('{{plugin_version}}', previewDevelopmentVersion)
}

function patchBuiltinMenuConfig(source: ConfigDocument) {
  return {
    ...source,
    builtin_features: {
      ...(source.builtin_features ?? {}),
      menu: {
        commands: normalizeTokens(draftCommands.value, defaultMenuCommands),
        prefixes: normalizeTokens(draftPrefixes.value),
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
  <AppPage :title="t('builtinFeatures.menuCenter.title')" :description="t('builtinFeatures.menuCenter.subtitle')" full-height>
    <template #extra>
      <div class="menu-center-actions">
        <a-button
          type="primary"
          :disabled="!hasUnsavedChanges"
          :loading="saving"
          data-testid="menu-center-save"
          @click="save"
        >
          <template #icon><SaveOutlined /></template>
          {{ t('builtinFeatures.menuCenter.save') }}
        </a-button>
        <a-button :loading="loading" @click="loadPage">
          <template #icon><ReloadOutlined /></template>
          {{ t('builtinFeatures.menuCenter.refresh') }}
        </a-button>
      </div>
    </template>

    <RetryPanel
      v-if="pageError && !configDocument"
      :title="t('errors.common.loadFailed')"
      :description="pageError"
      :loading="loading"
      @retry="loadPage"
    />

    <div v-else class="menu-center-layout">
      <div class="menu-center-float-panel">
        <div class="menu-center-float-panel__header">
          <span class="menu-center-float-panel__title">{{ t('builtinFeatures.menuCenter.title') }}</span>
          <a-tag v-if="hasUnsavedChanges" color="blue" class="menu-center-unsaved-tag">
            {{ t('builtinFeatures.menuCenter.unsaved') }}
          </a-tag>
        </div>

        <div class="menu-center-float-panel__body">
          <a-alert v-if="pageError" :message="pageError" type="error" show-icon class="menu-center-float-panel__alert" />

          <div class="menu-center-float-panel__field">
            <label class="menu-center-float-panel__label">{{ t('builtinFeatures.menuCenter.commands.label') }}</label>
            <a-select
              v-model:value="draftCommands"
              mode="tags"
              :token-separators="[',', '，', ' ']"
              :placeholder="t('builtinFeatures.menuCenter.commands.placeholder')"
              data-testid="menu-center-commands"
              class="menu-center-float-panel__select"
            />
          </div>

          <div class="menu-center-float-panel__field">
            <label class="menu-center-float-panel__label">{{ t('builtinFeatures.menuCenter.prefixes.label') }}</label>
            <a-select
              v-model:value="draftPrefixes"
              mode="tags"
              :token-separators="[',', '，', ' ']"
              :placeholder="t('builtinFeatures.menuCenter.prefixes.placeholder')"
              data-testid="menu-center-prefixes"
              class="menu-center-float-panel__select"
            />
            <div v-if="draftPrefixes.length === 0" class="menu-center-field-note" data-testid="menu-center-inherited-prefixes">
              {{ t('builtinFeatures.menuCenter.prefixes.inherited', { prefixes: inheritedPrefixLabel }) }}
            </div>
          </div>
        </div>
      </div>

      <div class="menu-preview-area">
        <a-tabs v-model:activeKey="activeTab" class="menu-center-tabs">
          <template #rightExtra>
            <a-select
              v-show="activeTab === 'plugin'"
              v-model:value="selectedPluginId"
              :options="pluginOptions"
              :placeholder="t('builtinFeatures.menuCenter.preview.allPlugins')"
              class="menu-center-plugin-select"
              size="small"
              data-testid="menu-center-plugin-select"
            />
          </template>

          <a-tab-pane key="root" :tab="t('builtinFeatures.menuCenter.preview.rootTitle')" force-render>
            <div class="menu-preview-card">
              <div class="menu-trigger-row">
                <span v-for="example in rootMenuTriggerExamples" :key="example" class="menu-trigger-chip">{{ example }}</span>
              </div>
              <NativeTemplatePreviewFrame
                template-id="help.menu"
                :data="rootPreviewData"
                data-testid="menu-center-root-preview"
              />
            </div>
          </a-tab-pane>

          <a-tab-pane key="plugin" :tab="t('builtinFeatures.menuCenter.preview.pluginTitle')" force-render>
            <div class="menu-preview-card">
              <template v-if="selectedPlugin">
                <NativeTemplatePreviewFrame
                  template-id="help.menu"
                  :data="selectedPluginPreviewData"
                  data-testid="menu-center-plugin-preview"
                />
              </template>
              <a-empty
                v-else
                :description="t('builtinFeatures.menuCenter.preview.noPlugins')"
                class="menu-preview-empty"
              />
            </div>
          </a-tab-pane>
        </a-tabs>
      </div>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
.menu-center-actions {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.menu-center-layout {
  --menu-center-panel-width: 260px;
  --menu-center-panel-inset: var(--space-md);
  --menu-center-preview-max-width: 1040px;
  --menu-center-preview-top-space: 0px;

  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  min-height: 0;
  flex: 1 1 auto;
  padding: var(--space-lg);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
}

.menu-center-float-panel {
  position: absolute;
  top: var(--menu-center-panel-inset);
  left: var(--menu-center-panel-inset);
  z-index: 10;
  width: var(--menu-center-panel-width);
  padding: var(--space-md);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: var(--shadow-elevated);
}

.menu-center-float-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
  padding-bottom: var(--space-sm);
  border-bottom: 1px solid var(--border);
}

.menu-center-float-panel__title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text);
}

.menu-center-unsaved-tag {
  font-size: 0.75rem;
  margin: 0;
}

.menu-center-float-panel__alert {
  margin-bottom: var(--space-md);

  :deep(.ant-alert-message) {
    font-size: 0.8rem;
  }
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
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--text);
  line-height: 1.4;
}

.menu-center-float-panel__select {
  width: 100%;
}

.menu-center-field-note {
  color: var(--muted);
  font-size: 0.75rem;
  line-height: 1.5;
}

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

  :deep(.ant-tabs-content) {
    flex: 1 1 auto;
    min-height: 0;
  }

  :deep(.ant-tabs-tabpane) {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  :deep(.ant-tabs-nav) {
    margin-bottom: var(--space-md);
  }

  :deep(.ant-tabs-tab) {
    font-weight: 500;
  }

  :deep(.ant-tabs-extra-content) {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }
}

.menu-center-plugin-select {
  width: 200px;
}

.menu-preview-card {
  display: flex;
  flex-direction: column;
  width: min(100%, var(--menu-center-preview-max-width));
  min-width: 0;
  margin-inline: auto;
  padding: var(--space-md);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
}

.menu-trigger-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
}

.menu-trigger-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 4px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 0.82rem;
  line-height: 1.4;
  word-break: break-all;
  transition: all 0.2s ease;
  cursor: default;

  &::before {
    content: '';
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    flex-shrink: 0;
  }

  &:hover {
    border-color: var(--border-accent);
    background: var(--surface-accent);
    transform: translateY(-1px);
    box-shadow: var(--shadow-sm);
  }
}

.menu-preview-empty {
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 320px;
}

:deep(.ant-tabs-extra-content) {
  .ant-select {
    font-size: 0.85rem;
  }

  .ant-select-selector {
    border-radius: var(--radius-sm) !important;
  }
}

@media (max-width: 1023px) {
  .menu-center-layout {
    --menu-center-preview-top-space: 0px;
  }

  .menu-center-float-panel {
    position: static;
    width: 100%;
    margin-bottom: var(--space-md);
    background: var(--surface);
  }
}

@media (max-width: 720px) {
  .menu-center-layout {
    padding: var(--space-sm);
  }

  .menu-center-float-panel {
    padding: var(--space-sm);
  }

  .menu-center-float-panel__body {
    gap: var(--space-sm);
  }

  .menu-trigger-row {
    margin-bottom: var(--space-sm);
  }

  .menu-center-tabs :deep(.ant-tabs-nav) {
    flex-wrap: wrap;
    row-gap: var(--space-sm);
  }

  .menu-center-tabs :deep(.ant-tabs-nav-wrap) {
    min-width: 0;
  }

  .menu-center-tabs :deep(.ant-tabs-extra-content) {
    width: 100%;
    margin-left: 0;
  }

  .menu-center-plugin-select {
    width: 100%;
  }

  .menu-preview-card {
    padding: var(--space-sm);
  }
}
</style>
