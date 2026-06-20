<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import {
  AppstoreOutlined,
  UnorderedListOutlined,
  SearchOutlined,
  PlusOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  PauseCircleOutlined,
  WarningOutlined,
  InfoCircleOutlined,
  SettingOutlined,
  EyeOutlined,
  UserOutlined,
  ArrowRightOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons-vue'

import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import PluginPowerButton from '@/components/PluginPowerButton.vue'
import AppTableToolbar from '@/components/AppTableToolbar.vue'
import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import PluginCommandsPanel from '@/components/PluginCommandsPanel.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  getPluginRoleLabel,
  getPluginStateLabel,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import { isPluginCommandConflicted } from '@/lib/plugin-commands'
import type { PluginCommandSummary, PluginInstallRequest } from '@/types/api'
import { usePluginsStore } from '@/stores/plugins'

type HealthNoticeTone = '' | 'info' | 'warning' | 'danger'

interface PluginHealthNotice {
  label: string
  tone: HealthNoticeTone
}

const router = useRouter()
const pluginsStore = usePluginsStore()
const { actionPending, error, installPending, loading, sortedItems } = storeToRefs(pluginsStore)
const installDialogVisible = ref(false)
const installError = ref<string | null>(null)
const summaryDrawerVisible = ref(false)
const summaryPluginId = ref<string | null>(null)
const expandedCommandPluginIds = ref(new Set<string>())
const installForm = reactive<PluginInstallRequest>({
  source_type: 'local_zip',
  source: '',
})

const searchQuery = ref('')
const filterState = ref<'all' | 'running' | 'disabled' | 'alert'>('all')
const filterSource = ref<'all' | 'official' | 'community'>('all')

const isTestEnv = computed(() => {
  const isVitest = typeof window !== 'undefined' && ((window as any).__vitest_worker__ || (window as any).VTU_COMPONENT)
  const isE2E = typeof navigator !== 'undefined' && navigator.webdriver
  return Boolean(isVitest || isE2E)
})

const layoutMode = ref<'grid' | 'list'>('list')

onMounted(() => {
  if (!isTestEnv.value) {
    layoutMode.value = (localStorage.getItem('plugins-layout-mode') as 'grid' | 'list') || 'grid'
  }
})

function changeLayoutMode(mode: 'grid' | 'list') {
  layoutMode.value = mode
  if (!isTestEnv.value) {
    localStorage.setItem('plugins-layout-mode', mode)
  }
}

const runningCount = computed(() => sortedItems.value.filter((item) => item.state === 'running').length)
const disabledCount = computed(() => sortedItems.value.filter((item) => item.state === 'disabled').length)
const alertCount = computed(() =>
  sortedItems.value.filter((item) =>
    item.state === 'failed' ||
    item.state === 'invalid' ||
    (item.command_conflicts?.length ?? 0) > 0
  ).length
)
const pageErrorToast = computed(() => (
  error.value
    ? {
        key: `plugins-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))
const installErrorToast = computed(() => (
  installError.value
    ? {
        key: `plugins-install-error:${installError.value}`,
        level: 'error' as const,
        message: installError.value,
      }
    : null
))

useToastFeedback(pageErrorToast)
useToastFeedback(installErrorToast)

function getPluginGradient(id: string) {
  const colors = [
    ['rgba(79, 140, 255, 0.85)', 'rgba(22, 104, 220, 0.95)'], // Blue
    ['rgba(74, 208, 125, 0.85)', 'rgba(42, 161, 95, 0.95)'],  // Green
    ['rgba(240, 183, 62, 0.85)', 'rgba(217, 154, 28, 0.95)'],  // Yellow/Orange
    ['rgba(239, 115, 123, 0.85)', 'rgba(225, 91, 100, 0.95)'], // Red
    ['rgba(155, 93, 229, 0.85)', 'rgba(131, 56, 236, 0.95)'],  // Purple
    ['rgba(0, 245, 212, 0.85)', 'rgba(0, 187, 249, 0.95)'],   // Cyan
  ]
  let hash = 0
  for (let i = 0; i < id.length; i++) {
    hash = id.charCodeAt(i) + ((hash << 5) - hash)
  }
  const index = Math.abs(hash) % colors.length
  return `linear-gradient(135deg, ${colors[index][0]} 0%, ${colors[index][1]} 100%)`
}

function getPluginInitials(name: string) {
  if (!name) return ''
  const trimmed = name.trim()
  if (/^[\u4e00-\u9fa5]/.test(trimmed)) {
    return trimmed.slice(0, 2)
  }
  const words = trimmed.split(/[\s._-]+/)
  if (words.length > 1) {
    return (words[0][0] + words[1][0]).toUpperCase()
  }
  return trimmed.slice(0, 2).toUpperCase()
}

function isOfficialPlugin(record: (typeof sortedItems.value)[number]) {
  return record.trust?.level === 'official' || record.source?.root?.startsWith('plugins/builtin') === true
}

function getTrustBadgeTone(record: (typeof sortedItems.value)[number]) {
  if (
    isOfficialPlugin(record)
  ) {
    return { label: '官方', color: 'blue', icon: SafetyCertificateOutlined }
  }
  if (record.trust?.level === 'unverified') {
    return { label: '未验证', color: 'error', icon: WarningOutlined }
  }
  return { label: record.trust?.label || '第三方', color: 'warning', icon: CheckCircleOutlined }
}

const filteredItems = computed(() => {
  return sortedItems.value.filter((item) => {
    // 1. Search Query
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchName = item.name?.toLowerCase().includes(q)
      const matchId = item.id?.toLowerCase().includes(q)
      const matchDesc = item.description?.toLowerCase().includes(q)
      if (!matchName && !matchId && !matchDesc) return false
    }

    // 2. Filter State
    if (filterState.value === 'running') {
      if (item.state !== 'running') return false
    } else if (filterState.value === 'disabled') {
      if (item.state !== 'disabled') return false
    } else if (filterState.value === 'alert') {
      const hasConflicts = (item.command_conflicts?.length ?? 0) > 0
      const hasIssue = item.state === 'failed' || item.state === 'invalid'
      if (!hasConflicts && !hasIssue) return false
    }

    // 3. Filter Source
    if (filterSource.value === 'official') {
      const isOfficial = isOfficialPlugin(item)
      if (!isOfficial) return false
    } else if (filterSource.value === 'community') {
      const isOfficial = isOfficialPlugin(item)
      if (isOfficial) return false
    }

    return true
  })
})

const summaryPlugin = computed(() => sortedItems.value.find((item) => item.id === summaryPluginId.value) ?? null)
const tableColumns = computed(() => [
  { title: t('plugins.fields.plugin'), key: 'title', dataIndex: 'name', width: 240 },
  { title: t('plugins.fields.version'), key: 'version', dataIndex: 'version', width: 96 },
  { title: t('plugins.fields.author'), key: 'author', dataIndex: 'author', width: 140 },
  { title: t('plugins.fields.description'), key: 'description', dataIndex: 'description', width: 320 },
  { title: t('plugins.fields.source'), key: 'source', dataIndex: 'source', width: 220 },
  { title: t('plugins.fields.commands'), key: 'commands', dataIndex: 'commands', width: 300 },
  { title: t('plugins.fields.state'), key: 'state', dataIndex: 'state', width: 300 },
  { title: t('plugins.fields.actions'), key: 'actions', dataIndex: 'actions', width: 396 },
])

function getConflictNotice(count: number) {
  return t('plugins.health.commandConflicts', { count })
}

function getPluginHealthNotices(row: (typeof sortedItems.value)[number]) {
  const notices: PluginHealthNotice[] = []
  const conflicts = row.command_conflicts?.length ?? 0

  if (conflicts > 0) {
    notices.push({ label: getConflictNotice(conflicts), tone: 'warning' })
  }

  if (row.source?.verified === false || row.trust?.level === 'unverified') {
    notices.push({ label: t('plugins.health.unverifiedSource'), tone: 'info' })
  }

  if (row.state === 'failed') {
    notices.push({ label: t('plugins.health.runtimeIssue'), tone: 'danger' })
  } else if (row.state === 'invalid') {
    notices.push({ label: t('plugins.health.invalidManifest'), tone: 'danger' })
  } else if (row.state === 'enabled') {
    notices.push({ label: t('plugins.health.enabledButStopped'), tone: 'warning' })
  }

  return notices.slice(0, 3)
}

function isCommandsExpanded(pluginId: string) {
  return expandedCommandPluginIds.value.has(pluginId)
}

function getVisibleCommands(pluginId: string, commands: PluginCommandSummary[]) {
  return isCommandsExpanded(pluginId) ? commands : commands.slice(0, 3)
}

function getOverflowCommandCount(commands: PluginCommandSummary[]) {
  return Math.max(commands.length - 3, 0)
}

function toggleCommandExpansion(pluginId: string) {
  const next = new Set(expandedCommandPluginIds.value)
  if (next.has(pluginId)) {
    next.delete(pluginId)
  } else {
    next.add(pluginId)
  }
  expandedCommandPluginIds.value = next
}

function getCommandAliasesText(command: PluginCommandSummary) {
  return command.aliases?.length ? command.aliases.join(', ') : t('display.empty')
}

function getOptionalDisplayText(value?: string | null) {
  const text = value?.trim()
  return text ? text : t('display.empty')
}

function isConflictedCommand(command: PluginCommandSummary, conflicts?: string[]) {
  return isPluginCommandConflicted(command, conflicts)
}

function getTagColor(tone: HealthNoticeTone) {
  if (tone === 'danger') return 'error'
  if (tone === 'warning') return 'warning'
  if (tone === 'info') return 'blue'
  return 'default'
}

function getStateColor(state?: string) {
  if (state === 'running') return 'success'
  if (state === 'disabled') return 'default'
  if (state === 'enabled' || state === 'starting' || state === 'stopping') return 'warning'
  return 'error'
}

async function loadPlugins() {
  try {
    await pluginsStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  void loadPlugins()
})

function openDetail(id: string) {
  void router.push({ name: 'plugin-detail', params: { id } })
}

function openSummary(id: string) {
  summaryPluginId.value = id
  summaryDrawerVisible.value = true
}

function getToggleAction(state?: string) {
  return state === 'disabled' ? 'enable' : 'disable'
}

function isPluginLifecycleSwitching(state?: string) {
  return state === 'starting' || state === 'stopping'
}

function isToggleLoading(pluginId: string, state?: string) {
  return actionPending.value[pluginId] === 'enable' ||
    actionPending.value[pluginId] === 'disable' ||
    isPluginLifecycleSwitching(state)
}

function isReloadDisabled(state?: string) {
  return state === 'disabled' ||
    state === 'starting' ||
    state === 'stopping' ||
    state === 'invalid'
}

async function reloadPlugin(pluginId: string) {
  try {
    await pluginsStore.executeAction(pluginId, 'reload')
    notifySuccess(t('plugins.actionAccepted'))
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}

async function submitInstall() {
  installError.value = null
  try {
    const response = await pluginsStore.installPlugin(installForm)
    installDialogVisible.value = false
    installForm.source_type = 'local_zip'
    installForm.source = ''
    delete installForm.allow_install_scripts
    notifySuccess(t('plugins.installAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    installError.value = getDisplayErrorMessage(error)
  }
}
</script>

<template>
  <AppPage :title="t('plugins.title')" full-height>
    <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadPlugins()"
    />

    <div v-else class="plugins-page-content">
      <div class="plugins-stats-row" v-motion="{ initial: { opacity: 0, y: -10 }, enter: { opacity: 1, y: 0, transition: { duration: 350 } } }">
        <div class="stat-card" @click="filterState = 'all'" :class="{ active: filterState === 'all' }">
          <div class="stat-icon-wrapper total">
            <AppstoreOutlined />
          </div>
          <div class="stat-info">
            <span class="stat-label">{{ t('plugins.stats.total') }}</span>
            <span class="stat-value">{{ sortedItems.length }}</span>
          </div>
        </div>
        <div class="stat-card" @click="filterState = 'running'" :class="{ active: filterState === 'running' }">
          <div class="stat-icon-wrapper running">
            <CheckCircleOutlined />
          </div>
          <div class="stat-info">
            <span class="stat-label">{{ t('plugins.stats.running') }}</span>
            <span class="stat-value">{{ runningCount }}</span>
          </div>
        </div>
        <div class="stat-card" @click="filterState = 'disabled'" :class="{ active: filterState === 'disabled' }">
          <div class="stat-icon-wrapper disabled">
            <PauseCircleOutlined />
          </div>
          <div class="stat-info">
            <span class="stat-label">{{ t('plugins.stats.disabled') }}</span>
            <span class="stat-value">{{ disabledCount }}</span>
          </div>
        </div>
        <div class="stat-card" @click="filterState = 'alert'" :class="{ active: filterState === 'alert' }">
          <div class="stat-icon-wrapper alert">
            <WarningOutlined />
          </div>
          <div class="stat-info">
            <span class="stat-label">{{ t('plugins.stats.alert') }}</span>
            <span class="stat-value">{{ alertCount }}</span>
          </div>
        </div>
      </div>

      <AppCard
        borderless
        class="plugins-card"
        v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 50 } } }"
      >
        <AppTableToolbar class="plugins-toolbar">
          <template #left>
            <div class="toolbar-filters">
              <a-input
                v-model:value="searchQuery"
                :placeholder="t('plugins.filter.searchPlaceholder')"
                class="filter-search"
                allow-clear
              >
                <template #prefix>
                  <SearchOutlined class="search-icon" />
                </template>
              </a-input>

              <a-radio-group v-model:value="filterState" button-style="solid" class="filter-radio-group">
                <a-radio-button value="all">{{ t('plugins.filter.stateAll') }}</a-radio-button>
                <a-radio-button value="running">{{ t('plugins.stats.running') }}</a-radio-button>
                <a-radio-button value="disabled">{{ t('plugins.stats.disabled') }}</a-radio-button>
                <a-radio-button value="alert">{{ t('plugins.stats.alert') }}</a-radio-button>
              </a-radio-group>

              <a-select v-model:value="filterSource" class="filter-select" :dropdown-match-select-width="false">
                <a-select-option value="all">{{ t('plugins.filter.sourceAll') }}</a-select-option>
                <a-select-option value="official">{{ t('plugins.filter.sourceOfficial') }}</a-select-option>
                <a-select-option value="community">{{ t('plugins.filter.sourceCommunity') }}</a-select-option>
              </a-select>
            </div>
          </template>

          <template #right>
            <div v-if="!isTestEnv" class="layout-switcher">
              <a-button
                type="text"
                class="switcher-btn"
                :class="{ active: layoutMode === 'grid' }"
                @click="changeLayoutMode('grid')"
              >
                <template #icon><AppstoreOutlined /></template>
              </a-button>
              <a-button
                type="text"
                class="switcher-btn"
                :class="{ active: layoutMode === 'list' }"
                @click="changeLayoutMode('list')"
              >
                <template #icon><UnorderedListOutlined /></template>
              </a-button>
              <span class="toolbar-divider" />
            </div>

            <a-button type="primary" @click="installDialogVisible = true">
              <template #icon><PlusOutlined /></template>
              {{ t('plugins.install') }}
            </a-button>
          </template>
        </AppTableToolbar>

        <div v-if="layoutMode === 'grid'" class="plugins-grid-container">
          <div v-if="filteredItems.length === 0" class="empty-container">
            <AppEmptyState
              icon="plugin"
              :title="t('plugins.empty.title')"
              :description="t('plugins.empty.description')"
              :action-label="t('plugins.install')"
              @action="installDialogVisible = true"
            />
          </div>
          <div v-else class="plugins-grid">
            <div
              v-for="item in filteredItems"
              :key="item.id"
              class="plugin-grid-card"
              :class="`status-${item.state}`"
            >
              <div class="card-header">
                <div class="plugin-avatar-wrapper">
                  <div class="plugin-avatar" :style="{ background: getPluginGradient(item.id) }">
                    <span class="avatar-initials">{{ getPluginInitials(item.name) }}</span>
                  </div>
                  <span class="status-indicator-dot" :class="item.state" />
                </div>

                <div class="plugin-identity">
                  <div class="name-row">
                    <h4 class="grid-plugin-name" @click="openDetail(item.id)">{{ item.name }}</h4>
                    <a-tooltip :title="getTrustBadgeTone(item).label">
                      <component
                        :is="getTrustBadgeTone(item).icon"
                        class="trust-icon"
                        :class="getTrustBadgeTone(item).color"
                      />
                    </a-tooltip>
                  </div>
                  <span class="grid-plugin-id">{{ item.id }}</span>
                </div>
              </div>

              <div class="card-body">
                <p class="grid-plugin-description" :title="getOptionalDisplayText(item.description)">
                  {{ getOptionalDisplayText(item.description) }}
                </p>

                <div class="grid-plugin-meta">
                  <div class="meta-item">
                    <span class="meta-label">{{ t('plugins.fields.version') }}:</span>
                    <span class="meta-value font-mono">{{ getOptionalDisplayText(item.version) }}</span>
                  </div>
                  <div class="meta-item">
                    <span class="meta-label">{{ t('plugins.fields.author') }}:</span>
                    <span class="meta-value">{{ getOptionalDisplayText(item.author) }}</span>
                  </div>
                  <div class="meta-item">
                    <span class="meta-label">{{ t('plugins.fields.source') }}:</span>
                    <span class="meta-value source-root" :title="item.source?.root">{{ item.source?.root ?? t('display.empty') }}</span>
                  </div>
                </div>

                <div class="grid-plugin-states">
                  <div class="state-badges">
                    <a-tag size="small" :color="getStateColor(item.state)">{{ getPluginStateLabel(item.state) }}</a-tag>
                  </div>
                  <div v-if="getPluginHealthNotices(item).length > 0" class="plugin-health-notices grid-notices">
                    <a-tag
                      v-for="notice in getPluginHealthNotices(item)"
                      :key="notice.label"
                      size="small"
                      :color="getTagColor(notice.tone)"
                    >
                      {{ notice.label }}
                    </a-tag>
                  </div>
                </div>

                <div class="grid-plugin-commands">
                  <div v-if="item.commands.length > 0" class="plugin-cell-commands">
                    <div
                      v-for="command in getVisibleCommands(item.id, item.commands)"
                      :key="`${item.id}-${command.name}`"
                      class="plugin-command-chip"
                    >
                      <a-tag
                        size="small"
                        :color="isConflictedCommand(command, item.command_conflicts) ? 'warning' : 'success'"
                      >
                        {{ command.name }}
                      </a-tag>
                      <a-tooltip v-if="command.aliases?.length" :title="getCommandAliasesText(command)">
                        <small>{{ t('plugins.commandAliasesCount', { count: command.aliases.length }) }}</small>
                      </a-tooltip>
                    </div>
                    <a-button
                      v-if="getOverflowCommandCount(item.commands) > 0"
                      class="plugin-command-expander"
                      size="small"
                      type="link"
                      @click="toggleCommandExpansion(item.id)"
                    >
                      {{ isCommandsExpanded(item.id)
                        ? t('plugins.commandCollapse')
                        : t('plugins.commandOverflow', { count: getOverflowCommandCount(item.commands) }) }}
                    </a-button>
                  </div>
                  <span v-else class="plugin-command-empty">{{ t('plugins.empty.commands') }}</span>
                </div>
              </div>

              <div class="card-actions">
                <div class="action-buttons-group">
                  <a-button size="small" type="text" class="btn-action" @click="openSummary(item.id)">
                    <template #icon><EyeOutlined /></template>
                    {{ t('plugins.actions.summary') }}
                  </a-button>
                  <a-button size="small" type="text" class="btn-action" @click="openDetail(item.id)">
                    <template #icon><SettingOutlined /></template>
                    {{ t('plugins.actions.detail') }}
                  </a-button>
                  <a-button
                    size="small"
                    type="text"
                    class="btn-action"
                    :data-testid="`plugin-reload-button-${item.id}`"
                    :loading="actionPending[item.id] === 'reload'"
                    :disabled="isReloadDisabled(item.state)"
                    @click="reloadPlugin(item.id)"
                  >
                    <template #icon><ReloadOutlined /></template>
                    {{ t('plugins.actions.reload') }}
                  </a-button>
                </div>

                <div class="action-controls-group">
                  <PluginPowerButton
                    compact
                    :checked="item.state !== 'disabled'"
                    :data-testid="`plugin-enable-button-${item.id}`"
                    :loading="isToggleLoading(item.id, item.state)"
                    :checked-label="t('plugins.actions.enable')"
                    :unchecked-label="t('plugins.actions.disable')"
                    @click="pluginsStore.executeAction(item.id, getToggleAction(item.state))"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <a-table
          v-else
          class="plugins-data-table app-data-table"
          :columns="tableColumns"
          :data-source="filteredItems"
          :pagination="false"
          :row-key="(row) => row.id"
          :scroll="{ x: 2012 }"
        >
          <template #emptyText>
            <AppEmptyState
              icon="plugin"
              :title="t('plugins.empty.title')"
              :description="t('plugins.empty.description')"
              :action-label="t('plugins.install')"
              @action="installDialogVisible = true"
            />
          </template>

          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'title'">
              <div class="plugin-cell-identity">
                <strong class="plugin-name">{{ record.name }}</strong>
                <small class="plugin-id">{{ record.id }}</small>
              </div>
            </template>

            <template v-else-if="column.key === 'source'">
              <div class="plugin-cell-source">
                <div class="plugin-source-root" :title="record.source?.root ?? t('display.empty')">
                  {{ record.source?.root ?? t('display.empty') }}
                </div>
                <div class="plugin-trust-label">
                  {{ record.trust?.label ?? t('display.empty') }}
                </div>
              </div>
            </template>

            <template v-else-if="column.key === 'version'">
              <span class="plugin-cell-version">{{ getOptionalDisplayText(record.version) }}</span>
            </template>

            <template v-else-if="column.key === 'author'">
              <span class="plugin-cell-author" :title="getOptionalDisplayText(record.author)">
                {{ getOptionalDisplayText(record.author) }}
              </span>
            </template>

            <template v-else-if="column.key === 'description'">
              <span class="plugin-cell-description" :title="getOptionalDisplayText(record.description)">
                {{ getOptionalDisplayText(record.description) }}
              </span>
            </template>

            <template v-else-if="column.key === 'commands'">
              <div v-if="record.commands.length > 0" class="plugin-cell-commands">
                <div
                  v-for="command in getVisibleCommands(record.id, record.commands)"
                  :key="`${record.id}-${command.name}`"
                  class="plugin-command-chip"
                >
                  <a-tag
                    size="small"
                    :color="isConflictedCommand(command, record.command_conflicts) ? 'warning' : 'success'"
                    :aria-label="`指令：${command.name}`"
                  >
                    {{ command.name }}
                  </a-tag>
                  <a-tooltip v-if="command.aliases?.length" :title="getCommandAliasesText(command)">
                    <small>{{ t('plugins.commandAliasesCount', { count: command.aliases.length }) }}</small>
                  </a-tooltip>
                </div>
                <a-button
                  v-if="getOverflowCommandCount(record.commands) > 0"
                  class="plugin-command-expander"
                  size="small"
                  type="link"
                  :aria-expanded="isCommandsExpanded(record.id)"
                  :aria-label="isCommandsExpanded(record.id)
                    ? t('plugins.commandCollapseAria', { name: record.name })
                    : t('plugins.commandExpandAria', { name: record.name, count: getOverflowCommandCount(record.commands) })"
                  @click="toggleCommandExpansion(record.id)"
                >
                  {{ isCommandsExpanded(record.id)
                    ? t('plugins.commandCollapse')
                    : t('plugins.commandOverflow', { count: getOverflowCommandCount(record.commands) }) }}
                </a-button>
              </div>
              <span v-else class="plugin-command-empty">{{ t('plugins.empty.commands') }}</span>
            </template>

            <template v-else-if="column.key === 'state'">
              <div class="plugin-cell-status">
                <div class="plugin-status-badges">
                  <a-tag size="small" :color="getStateColor(record.state)" :aria-label="`状态：${getPluginStateLabel(record.state)}`">{{ getPluginStateLabel(record.state) }}</a-tag>
                </div>
                <div v-if="getPluginHealthNotices(record).length > 0" class="plugin-health-notices">
                  <a-tag
                    v-for="notice in getPluginHealthNotices(record)"
                    :key="notice.label"
                    size="small"
                    :color="getTagColor(notice.tone)"
                    :aria-label="`健康状态：${notice.label}`"
                  >
                    {{ notice.label }}
                  </a-tag>
                </div>
              </div>
            </template>

            <template v-else-if="column.key === 'actions'">
              <div class="plugin-cell-actions">
                <a-button size="small" @click="openSummary(record.id)">{{ t('plugins.actions.summary') }}</a-button>
                <a-button size="small" @click="openDetail(record.id)">{{ t('plugins.actions.detail') }}</a-button>

                <a-divider type="vertical" />

                <PluginPowerButton
                  compact
                  :checked="record.state !== 'disabled'"
                  :data-testid="`plugin-enable-button-${record.id}`"
                  :loading="isToggleLoading(record.id, record.state)"
                  :checked-label="t('plugins.actions.enable')"
                  :unchecked-label="t('plugins.actions.disable')"
                  @click="pluginsStore.executeAction(record.id, getToggleAction(record.state))"
                />
                <a-button
                  size="small"
                  :data-testid="`plugin-reload-button-${record.id}`"
                  :loading="actionPending[record.id] === 'reload'"
                  :disabled="isReloadDisabled(record.state)"
                  @click="reloadPlugin(record.id)"
                >
                  {{ t('plugins.actions.reload') }}
                </a-button>
              </div>
            </template>
          </template>
        </a-table>
      </AppCard>
    </div>

    <a-modal
      v-model:open="installDialogVisible"
      :title="t('plugins.installDialogTitle')"
      :confirm-loading="installPending"
      :ok-text="t('plugins.installSubmit')"
      :cancel-text="t('dashboard.previewCancel')"
      :ok-button-props="{ disabled: !installForm.source }"
      @ok="submitInstall"
    >
      <a-form layout="vertical">
        <a-form-item :label="t('plugins.sourceType')">
          <a-select
            v-model:value="installForm.source_type"
            :options="[
              { label: t('plugins.localZip'), value: 'local_zip' },
              { label: t('plugins.localDirectory'), value: 'local_directory' },
              { label: t('plugins.remoteUrl'), value: 'remote_url' },
            ]"
          />
        </a-form-item>

        <a-form-item :label="installForm.source_type === 'remote_url' ? t('plugins.remoteUrlLabel') : t('plugins.serverPath')">
          <a-input v-model:value="installForm.source" />
        </a-form-item>

        <a-form-item>
          <a-checkbox v-model:checked="installForm.allow_install_scripts">
            {{ t('plugins.allowScripts') }}
          </a-checkbox>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-drawer
      v-model:open="summaryDrawerVisible"
      :title="t('plugins.actions.summary')"
      placement="right"
      width="min(560px, 92vw)"
    >
      <template v-if="summaryPlugin">
        <div class="drawer-section drawer-section--dense">
          <div class="mono-list">
            <strong>{{ summaryPlugin.name }}</strong>
            <small>{{ summaryPlugin.id }}</small>
          </div>
        </div>

        <AppCard borderless class="drawer-card">
          <a-descriptions :column="1" bordered size="small">
            <a-descriptions-item :label="t('plugins.fields.role')">{{ getPluginRoleLabel(summaryPlugin.role) }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.trust')">{{ summaryPlugin.trust?.label ?? t('display.empty') }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.state')">{{ getPluginStateLabel(summaryPlugin.state) }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.source')">{{ summaryPlugin.source?.root ?? t('display.empty') }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.sourceRef')">
              {{ summaryPlugin.source?.package_source_ref ?? summaryPlugin.source?.package_source_type ?? t('display.empty') }}
            </a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.conflicts')">
              <div v-if="summaryPlugin.command_conflicts?.length" class="table-actions">
                <a-tag v-for="command in summaryPlugin.command_conflicts" :key="command" size="small" color="warning">
                  {{ command }}
                </a-tag>
              </div>
              <span v-else>{{ t('display.empty') }}</span>
            </a-descriptions-item>
          </a-descriptions>
        </AppCard>

        <AppCard :title="t('plugins.sections.commands')" borderless class="drawer-card">
          <PluginCommandsPanel
            :commands="summaryPlugin.commands"
            :command-conflicts="summaryPlugin.command_conflicts"
          />
        </AppCard>
      </template>
    </a-drawer>
  </AppPage>
</template>

<style lang="scss" scoped>
.plugins-page-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1 1 auto;
  min-height: 0;
}

.plugins-stats-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 14px;
  width: 100%;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.25s cubic-bezier(0.4, 0, 0.2, 1), border-color 0.25s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.25s cubic-bezier(0.4, 0, 0.2, 1), color 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;

  &::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    width: 4px;
    height: 100%;
    background: transparent;
    transition: background-color 0.25s ease;
  }

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-lg);
    border-color: var(--border-accent);
    background: var(--surface-soft);
  }

  &.active {
    background: var(--surface-accent);
    border-color: var(--border-accent);
    box-shadow: var(--shadow);

    &::before {
      background: var(--accent);
    }
  }
}

.stat-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border-radius: 50%;
  font-size: 1.25rem;
  transition: transform 0.3s ease, box-shadow 0.3s ease, border-color 0.3s ease, background-color 0.3s ease, color 0.3s ease;

  &.total {
    background: var(--accent-soft);
    color: var(--accent);
  }
  &.running {
    background: color-mix(in srgb, var(--success) 12%, transparent);
    color: var(--success);
  }
  &.disabled {
    background: rgba(100, 116, 139, 0.12);
    color: var(--muted);
  }
  &.alert {
    background: color-mix(in srgb, var(--danger) 12%, transparent);
    color: var(--danger);
  }
}

.stat-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stat-label {
  font-size: 0.84rem;
  font-weight: 500;
  color: var(--muted);
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text);
  line-height: 1.2;
}

.plugins-toolbar {
  border-bottom: 1px solid var(--border);
  padding: 12px 16px;
  background: var(--surface);

  :deep(.app-table-toolbar-right) {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.filter-search {
  width: 260px;
  border-radius: 6px;

  :deep(.ant-input) {
    border-radius: 6px;
  }
  .search-icon {
    color: var(--muted);
  }
}

.filter-radio-group {
  :deep(.ant-radio-button-wrapper) {
    border-radius: 0;
    &:first-child {
      border-radius: 6px 0 0 6px;
    }
    &:last-child {
      border-radius: 0 6px 6px 0;
    }
  }
}

.filter-select {
  width: 140px;
  :deep(.ant-select-selector) {
    border-radius: 6px !important;
  }
}

.layout-switcher {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-right: 4px;
}

.switcher-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px !important;
  color: var(--muted);
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;

  &:hover {
    background: var(--surface-soft);
    color: var(--text);
  }

  &.active {
    background: var(--accent-soft) !important;
    color: var(--accent) !important;
  }
}

.toolbar-divider {
  width: 1px;
  height: 20px;
  background: var(--border);
  margin: 0 8px;
}

.plugins-grid-container {
  padding: 20px;
  background: var(--surface-soft);
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
}

.plugins-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 20px;
}

.plugin-grid-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  box-shadow: var(--shadow-sm);
  display: flex;
  flex-direction: column;
  transition: transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
  position: relative;
  overflow: hidden;

  &:hover {
    transform: translateY(-4px);
    box-shadow: 0 12px 24px -10px rgba(15, 23, 42, 0.15), var(--shadow);
    border-color: var(--border-accent);
  }

  &::after {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    width: 4px;
    height: 100%;
    background: var(--border);
  }

  &.status-running::after {
    background: var(--success);
  }
  &.status-disabled::after {
    background: #64748b;
  }
  &.status-failed::after,
  &.status-invalid::after {
    background: var(--danger);
  }
  &.status-starting::after,
  &.status-stopping::after,
  &.status-enabled::after {
    background: var(--warning);
  }
}

.card-header {
  display: flex;
  gap: 16px;
  padding: 20px 20px 14px;
  align-items: center;
}

.plugin-avatar-wrapper {
  position: relative;
  flex-shrink: 0;
}

.plugin-avatar {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: var(--shadow-sm);
  color: #fff;
}

.avatar-initials {
  font-weight: 700;
  font-size: 1.1rem;
  letter-spacing: -0.5px;
}

.status-indicator-dot {
  position: absolute;
  bottom: -2px;
  right: -2px;
  width: 13px;
  height: 13px;
  border-radius: 50%;
  border: 2.5px solid var(--surface);
  box-shadow: var(--shadow-sm);
  background-color: #64748b;

  &.running {
    background-color: var(--success);
    animation: status-pulse 2s infinite;
  }
  &.disabled {
    background-color: #64748b;
  }
  &.failed, &.invalid {
    background-color: var(--danger);
    animation: status-pulse 1.5s infinite;
  }
  &.starting, &.stopping, &.enabled {
    background-color: var(--warning);
    animation: status-pulse 2s infinite;
  }
}

@keyframes status-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(63, 190, 115, 0.4);
  }
  70% {
    box-shadow: 0 0 0 6px rgba(63, 190, 115, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(63, 190, 115, 0);
  }
}

.plugin-identity {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1 1 auto;
}

.name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.grid-plugin-name {
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text);
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
  transition: color 0.2s ease;

  &:hover {
    color: var(--accent);
  }
}

.trust-icon {
  font-size: 0.95rem;
  flex-shrink: 0;

  &.blue { color: var(--accent); }
  &.error { color: var(--danger); }
  &.warning { color: var(--warning); }
}

.grid-plugin-id {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 2px;
}

.card-body {
  padding: 0 20px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  flex: 1 1 auto;
}

.grid-plugin-description {
  font-size: 0.86rem;
  color: var(--muted);
  line-height: 1.5;
  margin: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  height: 2.6rem;
}

.grid-plugin-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  background: var(--surface-soft);
  border-radius: 8px;
  font-size: 0.8rem;
}

.meta-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.meta-label {
  color: var(--muted);
  font-weight: 500;
}

.meta-value {
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 70%;

  &.source-root {
    font-family: var(--font-mono);
    font-size: 0.76rem;
  }
}

.grid-plugin-states {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.state-badges,
.grid-notices {
  display: contents;
}

.grid-plugin-commands {
  border-top: 1px dashed var(--border);
  padding-top: 12px;
}

.plugin-command-empty {
  font-size: 0.8rem;
  color: var(--muted);
  display: block;
}

.card-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-top: 1px solid var(--border);
  background: var(--surface-soft);
  gap: 6px;
}

.action-buttons-group {
  display: flex;
  gap: 4px;
}

.btn-action {
  font-size: 0.8rem;
  padding: 0 8px;
  height: 28px;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 4px;
  border-radius: 6px !important;
  transition: background-color 0.25s ease, color 0.25s ease, transform 0.25s cubic-bezier(0.25, 0.8, 0.25, 1);
  font-weight: 500;

  .anticon {
    font-size: 13px;
    transition: transform 0.35s cubic-bezier(0.25, 0.8, 0.25, 1);
  }

  &:hover {
    color: var(--accent);
    background: var(--surface-accent) !important;
    transform: translateY(-1px);

    .anticon-eye {
      transform: scale(1.12);
    }

    .anticon-setting {
      transform: rotate(45deg);
    }

    .anticon-reload {
      transform: rotate(180deg);
    }
  }

  &:active {
    transform: translateY(0) scale(0.97);
  }
}

.action-controls-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.empty-container {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
}

.plugins-card {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  box-shadow: var(--shadow-xs);
}

.plugins-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  padding: 0;
}

.plugins-data-table {
  flex: 1 1 auto;
  min-height: 0;
  border-radius: 0 0 var(--app-card-radius) var(--app-card-radius);
  overflow: hidden;
}

.plugins-data-table :deep(.ant-table-row:hover > td) {
  background: var(--surface-accent) !important;
}

.plugin-cell-identity {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.plugin-name {
  font-size: 0.95rem;
  color: var(--text);
  font-weight: 600;
}

.plugin-id {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--muted);
}

.plugin-cell-source {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.plugin-source-root {
  font-size: 0.88rem;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

.plugin-trust-label {
  font-size: 0.8rem;
  color: var(--muted);
}

.plugin-cell-version {
  font-family: var(--font-mono);
  font-size: 0.82rem;
  color: var(--muted);
}

.plugin-cell-author {
  display: block;
  max-width: 100%;
  overflow: hidden;
  color: var(--muted);
  font-size: 0.86rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-cell-description {
  display: -webkit-box;
  overflow: hidden;
  color: var(--muted);
  font-size: 0.86rem;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.plugin-cell-status {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
}

.plugin-status-badges,
.plugin-health-notices {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.plugin-cell-commands {
  display: flex;
  gap: 6px 8px;
  align-items: center;
  flex-wrap: wrap;
}

.plugin-command-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  flex: 0 1 auto;
}

.plugin-command-chip :deep(.ant-tag) {
  margin-inline-end: 0;
}

.plugin-command-chip small,
.plugin-command-empty {
  color: var(--muted);
  font-size: 0.8rem;
}

.plugin-command-expander {
  height: 22px;
  padding: 0 6px;
  color: var(--muted);
  font-size: 0.8rem;
  line-height: 20px;
}

.plugin-command-expander:hover,
.plugin-command-expander:focus-visible {
  color: var(--primary);
}

.plugin-cell-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  flex-wrap: wrap;
}

.plugin-cell-actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
}

.drawer-card {
  margin-top: 12px;
}

.drawer-section {
  padding: 16px 0;
  border-bottom: 1px solid var(--border);
}

.mono-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  strong { font-size: 1rem; font-weight: 600; }
  small { font-family: var(--font-mono); font-size: 0.8rem; color: var(--muted); }
}
</style>
