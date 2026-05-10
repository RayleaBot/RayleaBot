<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ReloadOutlined, SaveOutlined } from '@ant-design/icons-vue'

import { notifySuccess } from '@/adapter/feedback'
import AppCard from '@/components/AppCard.vue'
import NativeTemplatePreviewFrame from '@/components/NativeTemplatePreviewFrame.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getPrimaryCommandPrefix } from '@/lib/command-usage'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import type { ConfigDocument, PluginCommandSummary, PluginHelpItem, PluginSummary } from '@/types/api'

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
    usage: pluginMenuTrigger(plugin),
  })))

const selectedPluginPreviewGroups = computed(() => {
  if (!selectedPlugin.value) {
    return []
  }

  const commandItems = selectedPlugin.value.commands.map((command) => ({
    name: command.name,
    description: command.description || command.name,
    usage: formatCommandUsage(command),
    permission: command.permission || 'everyone',
  }))
  const groups = commandItems.length > 0
    ? [{ title: '命令', items: commandItems }]
    : []

  for (const group of selectedPlugin.value.help?.groups ?? []) {
    const items = group.items.map((item) => ({
      name: item.command || item.title,
      description: item.description || item.title,
      usage: item.usage || formatHelpCommandUsage(item),
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
    groups: selectedPluginPreviewGroups.value,
    render_footer: nativeMenuPreviewFooter.value,
  }
})

const inheritedPrefixLabel = computed(() => inheritedCommandPrefixes.value.join('、'))
const nativeMenuPreviewFooter = computed(() => renderNativeMenuPreviewFooter(configDocument.value?.render?.footer_template))

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

function pluginMenuTrigger(plugin: PluginSummary) {
  const command = draftCommands.value[0] || defaultMenuCommands[0]
  return `${primaryMenuPrefix.value}${command} ${plugin.name || plugin.id}`
}

function suffixMenuTrigger(plugin: PluginSummary) {
  return `${primaryMenuPrefix.value}${plugin.name || plugin.id}${draftCommands.value[1] || draftCommands.value[0] || defaultMenuCommands[1]}`
}

function rootMenuTrigger(command: string) {
  return `${primaryMenuPrefix.value}${command}`
}

function formatCommandUsage(command: PluginCommandSummary) {
  const commandName = command.name.trim()
  if (!commandName) {
    return ''
  }
  const usage = command.usage?.trim()
  if (!usage) {
    return `${primaryMenuPrefix.value}${commandName}`
  }
  return usage.startsWith(primaryMenuPrefix.value) ? usage : `${primaryMenuPrefix.value}${usage}`
}

function formatHelpCommandUsage(item: PluginHelpItem) {
  const command = item.command?.trim()
  if (!command) {
    return item.usage ?? ''
  }
  return item.usage?.trim() || `${primaryMenuPrefix.value}${command}`
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
      <AppCard borderless shadow="sm" class="menu-center-config">
        <div class="menu-center-alerts">
          <a-alert v-if="pageError" :message="t('errors.common.loadFailed')" :description="pageError" type="error" show-icon class="menu-center-alert" />
          <a-alert v-if="hasUnsavedChanges" :message="t('builtinFeatures.menuCenter.unsaved')" type="info" show-icon class="menu-center-alert" />
        </div>

        <a-form layout="vertical">
          <a-form-item :label="t('builtinFeatures.menuCenter.commands.label')">
            <a-select
              v-model:value="draftCommands"
              mode="tags"
              :token-separators="[',', '，', ' ']"
              :placeholder="t('builtinFeatures.menuCenter.commands.placeholder')"
              data-testid="menu-center-commands"
            />
          </a-form-item>

          <a-form-item :label="t('builtinFeatures.menuCenter.prefixes.label')">
            <a-select
              v-model:value="draftPrefixes"
              mode="tags"
              :token-separators="[',', '，', ' ']"
              :placeholder="t('builtinFeatures.menuCenter.prefixes.placeholder')"
              data-testid="menu-center-prefixes"
            />
            <div v-if="draftPrefixes.length === 0" class="menu-center-field-note" data-testid="menu-center-inherited-prefixes">
              {{ t('builtinFeatures.menuCenter.prefixes.inherited', { prefixes: inheritedPrefixLabel }) }}
            </div>
          </a-form-item>
        </a-form>
      </AppCard>

      <div class="menu-center-preview-area">
        <div class="menu-center-preview-grid">
          <AppCard borderless :title="t('builtinFeatures.menuCenter.preview.rootTitle')" shadow="sm" class="menu-preview-card">
            <div class="menu-trigger-row">
              <span v-for="command in draftCommands" :key="command" class="menu-trigger-chip">{{ rootMenuTrigger(command) }}</span>
            </div>
            <NativeTemplatePreviewFrame
              template-id="help.menu"
              :data="rootPreviewData"
              data-testid="menu-center-root-preview"
            />
          </AppCard>

          <AppCard borderless :title="t('builtinFeatures.menuCenter.preview.pluginTitle')" shadow="sm" class="menu-preview-card">
            <template #extra>
              <a-select
                v-model:value="selectedPluginId"
                :options="pluginOptions"
                :placeholder="t('builtinFeatures.menuCenter.preview.allPlugins')"
                style="width: 180px;"
                size="small"
                data-testid="menu-center-plugin-select"
              />
            </template>

            <div v-if="selectedPlugin" class="menu-trigger-row">
              <span class="menu-trigger-chip">{{ pluginMenuTrigger(selectedPlugin) }}</span>
              <span class="menu-trigger-chip">{{ suffixMenuTrigger(selectedPlugin) }}</span>
            </div>

            <NativeTemplatePreviewFrame
              template-id="help.menu"
              :data="selectedPluginPreviewData"
              data-testid="menu-center-plugin-preview"
            />
          </AppCard>
        </div>
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

/* Main layout: config panel + preview area */
.menu-center-layout {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
  gap: var(--space-xl);
  min-height: 0;
}

/* Config panel */
.menu-center-config {
  min-width: 0;
  border-left: 3px solid var(--accent);

  :deep(.ant-card-body) {
    padding: var(--space-lg);
  }
}

/* Alert group */
.menu-center-alerts {
  display: grid;
  gap: var(--space-sm);
  margin-bottom: var(--space-lg);
}

.menu-center-alert {
  margin-bottom: 0;
  border-radius: var(--radius-sm);

  :deep(.ant-alert-message) {
    font-weight: 500;
  }
}

/* Field note */
.menu-center-field-note {
  margin-top: var(--space-sm);
  color: var(--muted);
  font-size: 0.84rem;
  line-height: 1.5;
}

/* Preview area wrapper */
.menu-center-preview-area {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

/* Preview grid: 2 columns */
.menu-center-preview-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-lg);
  min-width: 0;
}

/* Preview cards */
.menu-preview-card {
  min-width: 0;
  background: linear-gradient(180deg, var(--surface-strong) 0%, var(--surface-soft) 100%);
  border-radius: var(--radius-lg);

  :deep(.ant-card-body) {
    padding: var(--space-md);
  }

  /* Darken iframe border to blend with dark preview content */
  :deep(.native-template-preview) {
    border-color: rgba(255, 255, 255, 0.06);
  }
}

/* Plugin selector in card header */
.menu-preview-card :deep(.app-card__extra) {
  .ant-select {
    font-size: 0.85rem;
  }

  .ant-select-selector {
    border-radius: var(--radius-sm) !important;
  }
}

/* Trigger chips row */
.menu-trigger-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
}

/* Polished trigger chips */
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

/* Responsive: medium screens */
@media (max-width: 1399px) {
  .menu-center-layout {
    grid-template-columns: minmax(260px, 300px) minmax(0, 1fr);
    gap: var(--space-lg);
  }
}

/* Responsive: tablet and below */
@media (max-width: 1023px) {
  .menu-center-layout {
    grid-template-columns: 1fr;
  }

  .menu-center-preview-grid {
    grid-template-columns: 1fr;
  }

  .menu-center-config {
    border-left: 0;
    border-top: 3px solid var(--accent);
  }
}

/* Responsive: mobile */
@media (max-width: 720px) {
  .menu-center-layout {
    gap: var(--space-md);
  }

  .menu-center-preview-grid {
    gap: var(--space-md);
  }

  .menu-preview-card :deep(.ant-card-body) {
    padding: var(--space-sm);
  }

  .menu-center-config :deep(.ant-card-body) {
    padding: var(--space-md);
  }

  .menu-preview-card :deep(.app-card__extra) {
    width: 100%;
    margin-top: var(--space-sm);

    .ant-select {
      width: 100% !important;
    }
  }
}
</style>
