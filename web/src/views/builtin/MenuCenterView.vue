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
import NativeTemplatePreviewFrame from '@/components/NativeTemplatePreviewFrame.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getPrimaryCommandPrefix } from '@/lib/command-usage'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import { usePluginCollection } from '@/lib/use-plugin-collection'
import type {
  CommandPermissionLevel,
  ConfigDocument,
  PluginCommandSummary,
  PluginSummary,
} from '@/types/api'

const defaultMenuCommands = ['help', '帮助']
const defaultRenderFooterTemplate = 'Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}'
const previewDevelopmentVersion = '开发版本'
const previewSystemMenuPluginName = 'RayleaBot'

type CommandUsagePartKind = 'literal' | 'required' | 'optional'

interface CommandUsagePart {
  kind: CommandUsagePartKind
  text: string
}

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
const pageErrorToast = computed(() => (
  pageError.value && configDocument.value
    ? {
        key: `menu-center-error:${pageError.value}`,
        level: 'error' as const,
        message: pageError.value,
      }
    : null
))

useToastFeedback(pageErrorToast)
const inheritedCommandPrefixes = computed(() => normalizeTokens(configDocument.value?.command?.prefixes).length > 0
  ? normalizeTokens(configDocument.value?.command?.prefixes)
  : ['/'])
const effectiveMenuPrefixes = computed(() => draftPrefixes.value.length > 0 ? draftPrefixes.value : inheritedCommandPrefixes.value)
const primaryMenuPrefix = computed(() => getPrimaryCommandPrefix(effectiveMenuPrefixes.value))

const enabledPlugins = computed(() => sortedItems.value
  .filter((plugin) => plugin.state === 'running')
  .sort((left, right) => compareLabel(left.name, right.name) || compareLabel(left.id, right.id)))

const selectedPlugin = computed(() => (
  pluginsStore.knownItems.find((plugin) => plugin.id === selectedPluginId.value && plugin.state === 'running')
    ?? enabledPlugins.value.find((plugin) => plugin.id === selectedPluginId.value)
    ?? null
))

const rootPreviewItems = computed(() => enabledPlugins.value
  .map((plugin) => ({
    name: plugin.name || plugin.id,
    description: plugin.help?.summary || plugin.commands[0]?.description || plugin.id,
  })))

const selectedPluginPreviewGroups = computed(() => {
  const plugin = selectedPlugin.value
  if (!plugin) {
    return []
  }

  const commandByID = new Map(plugin.commands.map((command) => [command.id, command]))
  const covered = new Set<string>()
  const groups: Array<{ title: string, items: Array<Record<string, unknown>> }> = []
  for (const group of plugin.command_groups) {
    const items = group.commands.flatMap((commandID) => {
      const command = commandByID.get(commandID)
      if (!command) {
        return []
      }
      covered.add(command.id)
      return [commandPreviewItem(command)]
    })
    if (items.length > 0) {
      groups.push({ title: group.title, items })
    }
  }

  const ungrouped = plugin.commands
    .filter((command) => !covered.has(command.id))
    .map(commandPreviewItem)
  if (ungrouped.length > 0) {
    groups.push({ title: '其他命令', items: ungrouped })
  }

  return groups
})

const rootPreviewData = computed(() => ({
  title: '插件菜单',
  subtitle: '当前可用插件',
  command_prefixes: effectiveMenuPrefixes.value,
  trigger_examples: rootMenuTriggerExamples.value,
  items: rootPreviewItems.value,
  render_footer: renderNativeMenuPreviewFooter(configDocument.value?.render?.footer_template),
}))

const selectedPluginPreviewData = computed(() => {
  const plugin = selectedPlugin.value
  if (!plugin) {
    return {
      title: '插件菜单',
      subtitle: '当前没有可预览的插件菜单。',
      groups: [],
      render_footer: renderNativeMenuPreviewFooter(configDocument.value?.render?.footer_template),
    }
  }
  return {
    title: plugin.name || plugin.id,
    subtitle: plugin.help?.summary || plugin.commands[0]?.description || plugin.id,
    command_prefixes: effectiveMenuPrefixes.value,
    groups: selectedPluginPreviewGroups.value,
    render_footer: renderNativeMenuPreviewFooter(configDocument.value?.render?.footer_template, plugin),
  }
})

const inheritedPrefixLabel = computed(() => inheritedCommandPrefixes.value.join('、'))
const rootMenuTriggerExamples = computed(() => buildMenuTriggerExamples(enabledPlugins.value[0] ?? null))

const hasUnsavedChanges = computed(() => {
  const source = configDocument.value
  if (!source) {
    return false
  }
  return JSON.stringify(draftCommands.value) !== JSON.stringify(normalizeTokens(source.builtin_features?.menu?.commands, defaultMenuCommands))
    || JSON.stringify(draftPrefixes.value) !== JSON.stringify(normalizeTokens(source.builtin_features?.menu?.prefixes))
})

watch(configDocument, (value, previous) => {
  if (value && previous && JSON.stringify(value) === JSON.stringify(previous)) return
  draftCommands.value = normalizeTokens(value?.builtin_features?.menu?.commands, defaultMenuCommands)
  draftPrefixes.value = normalizeTokens(value?.builtin_features?.menu?.prefixes)
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

function renderNativeMenuPreviewFooter(template?: string, plugin?: Pick<PluginSummary, 'id' | 'name' | 'version'> | null) {
  const source = template?.trim() || defaultRenderFooterTemplate
  const pluginName = plugin ? plugin.name || plugin.id : previewSystemMenuPluginName
  const pluginVersion = displayPreviewVersion(plugin?.version)
  return source
    .replaceAll('{{rayleabot_version}}', previewDevelopmentVersion)
    .replaceAll('{{plugin_name}}', pluginName)
    .replaceAll('{{plugin_version}}', pluginVersion)
}

function displayPreviewVersion(version?: string | null) {
  const normalized = String(version ?? '').trim()
  return normalized && normalized !== '0.0.0-dev' ? normalized : previewDevelopmentVersion
}

function commandPreviewItem(command: PluginCommandSummary) {
  return {
    name: command.effective_names[0] || command.name,
    ...commandPreviewFields(command),
    description: command.description || command.name,
    permission: effectiveCommandPermission(command),
  }
}

function commandPreviewFields(command: PluginCommandSummary) {
  if (command.trigger.type === 'pattern') {
    const usage = patternCommandUsage(command.usage, effectiveMenuPrefixes.value)
    return {
      trigger_type: command.trigger.type,
      command_prefixes: effectiveMenuPrefixes.value,
      usage,
      usage_parts: commandUsageParts(usage, 'literal'),
    }
  }
  const commandName = command.effective_names[0] || command.name
  const usageArgs = commandUsageArgs(commandName, command.usage, effectiveMenuPrefixes.value)
  return {
    trigger_type: command.trigger.type,
    command_prefixes: effectiveMenuPrefixes.value,
    ...(usageArgs
      ? {
          usage_args: usageArgs,
          usage_parts: commandUsageParts(usageArgs, 'required'),
        }
      : {}),
  }
}

function effectiveCommandPermission(command: PluginCommandSummary): CommandPermissionLevel {
  const declaredPermission = String(command.permission ?? '').trim()
  if (declaredPermission) {
    return normalizeCommandPermission(declaredPermission)
  }
  return normalizeCommandPermission(configDocument.value?.permission?.default_level)
}

function normalizeCommandPermission(value: unknown): CommandPermissionLevel {
  switch (String(value ?? '').trim()) {
    case 'super_admin':
      return 'super_admin'
    case 'group_admin':
      return 'group_admin'
    case 'everyone':
      return 'everyone'
    default:
      return 'everyone'
  }
}

function patternCommandUsage(usage: string | null | undefined, prefixes: string[]) {
  const value = String(usage ?? '').trim()
  if (!value) {
    return ''
  }
  return stripCommandExamplePrefix(value, prefixes)
}

function stripCommandExamplePrefix(value: string, prefixes: string[]) {
  const examplePrefixes = [...new Set([...prefixes, '/', '#', '*', '＊'])]
    .map((prefix) => prefix.trim())
    .filter(Boolean)
    .sort((left, right) => right.length - left.length)
  const matchedPrefix = examplePrefixes.find((prefix) => value.startsWith(prefix))
  return matchedPrefix ? value.slice(matchedPrefix.length).trimStart() : value
}

function commandUsageArgs(commandName?: string | null, usage?: string | null, prefixes: string[] = []) {
  const command = String(commandName ?? '').trim()
  let value = String(usage ?? '').trim()
  if (!command || !value) {
    return ''
  }
  value = stripCommandExamplePrefix(value, prefixes)
  if (value === command) {
    return ''
  }
  if (value.startsWith(command)) {
    return value.slice(command.length).trim()
  }
  return ''
}

function commandUsageParts(usage: string, plainKind: 'literal' | 'required'): CommandUsagePart[] {
  const source = usage.trim()
  if (!source) {
    return []
  }

  const parts: CommandUsagePart[] = []
  const pattern = /\[([^\]]+)\]|<([^>]+)>/g
  let cursor = 0
  for (const match of source.matchAll(pattern)) {
    const index = match.index ?? cursor
    appendCommandUsagePart(parts, plainKind, source.slice(cursor, index))
    appendCommandUsagePart(parts, match[1] === undefined ? 'required' : 'optional', match[1] ?? match[2] ?? '')
    cursor = index + match[0].length
  }
  appendCommandUsagePart(parts, plainKind, source.slice(cursor))
  return parts
}

function appendCommandUsagePart(parts: CommandUsagePart[], kind: CommandUsagePartKind, text: string) {
  const normalized = text.trim()
  if (normalized) {
    parts.push({ kind, text: normalized })
  }
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
  <AppPage :title="t('builtinFeatures.menuCenter.title')" :show-header="false" full-height>
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
              <div class="menu-trigger-row">
                <span v-for="example in rootMenuTriggerExamples" :key="example" class="menu-trigger-chip">{{ example }}</span>
              </div>
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
@use '@/styles/breakpoints.generated' as bp;
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
  --menu-center-panel-width: 260px;
  --menu-center-panel-inset: var(--space-md);
  --menu-center-preview-max-width: 1040px;
  --menu-center-preview-top-space: 0px;

  display: grid;
  grid-template-columns: var(--menu-center-panel-width) minmax(0, 1fr);
  gap: var(--space-lg);
  min-height: 0;
  flex: 1 1 auto;
  padding: var(--space-lg);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
}

.menu-center-float-panel {
  width: 100%;
  align-self: start;
  padding: var(--space-md);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-strong);
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

.menu-preview-area {
  min-width: 0;
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 0;
  padding-top: var(--menu-center-preview-top-space);
}

.menu-preview-area :deep(.native-template-preview__frame) {
  transform-origin: left top;
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
  box-shadow: none;
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
  transition: border-color 150ms ease, background-color 150ms ease, color 150ms ease;
  cursor: default;

  &::before {
    content: '';
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--brand-fill);
    flex-shrink: 0;
  }

}

.menu-preview-empty {
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 320px;
}

:deep(.app-tabs__extra) {
  .app-select-wrap {
    font-size: 0.85rem;
  }

  .app-select {
    border-radius: var(--radius-sm) !important;
  }
}

@media (max-width: #{bp.$navigation - 1px}) {
  .menu-center-layout {
    --menu-center-preview-top-space: 0px;
    grid-template-columns: minmax(0, 1fr);
  }

  .menu-center-float-panel {
    width: 100%;
    margin-bottom: var(--space-md);
    background: var(--surface);
  }
}

@media (max-width: #{bp.$compactPanel}) {
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

  .menu-center-tabs :deep(.app-tabs__header) {
    flex-wrap: wrap;
    row-gap: var(--space-sm);
  }

  .menu-center-tabs :deep(.app-tabs__list) {
    min-width: 0;
  }

  .menu-center-tabs :deep(.app-tabs__extra) {
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
