<script setup lang="ts">
import {
  ArrowLeftOutlined,
  DownOutlined,
  LoadingOutlined,
  ReloadOutlined,
  RightOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue'
import { computed, nextTick, ref, shallowRef, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, type RouteLocationRaw } from 'vue-router'

import { resolveMenuIcon } from '@/access/icons'
import type { AppMenuItem } from '@/access/menu'
import {
  isPluginCenterRoute,
  isPluginWorkspaceRoute,
  pluginCenterPageGroups,
  pluginCenterPages,
  pluginCenterTabName,
} from '@/access/plugin-center'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import { getPluginStateLabel } from '@/lib/display'
import {
  buildPluginDetailLocation,
  readPluginDetailPanel,
  readPluginManagementPage,
} from '@/lib/management-links'
import { t } from '@/i18n'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail, PluginState, PluginSummary } from '@/types/api'

type NavigationScope = 'root' | 'plugin-center'

interface OpenPluginTarget {
  fullPath: string
  pluginId: string
}

type SidebarPluginItem = Pick<PluginSummary, 'id' | 'name'> & {
  icon?: PluginSummary['icon']
  state?: PluginState
  version?: PluginSummary['version']
}

type PluginManagementPage = NonNullable<PluginDetail['management_ui']>['pages'][number]

type SidebarPluginNavigationEntry =
  | { key: string; kind: 'resource'; plugin: SidebarPluginItem }
  | { key: string; kind: 'overview'; plugin: SidebarPluginItem }
  | { key: string; kind: 'management'; page: PluginManagementPage; plugin: SidebarPluginItem }
  | { key: string; kind: 'retry'; plugin: SidebarPluginItem }

const props = withDefaults(defineProps<{
  collapsed?: boolean
  menuItems: AppMenuItem[]
  mobile?: boolean
  openKeys: string[]
  openPluginTargets?: OpenPluginTarget[]
  scope: NavigationScope
  selectedKeys: string[]
}>(), {
  collapsed: false,
  mobile: false,
  openPluginTargets: () => [],
})

const emit = defineEmits<{
  navigate: [target: RouteLocationRaw]
  openChange: [keys: string[]]
  scopeChange: [scope: NavigationScope]
}>()

const route = useRoute()
const pluginsStore = usePluginsStore()
const {
  current,
  detailErrorsByPluginId,
  detailLoadingByPluginId,
  detailsByPluginId,
  error,
  loading,
  sortedItems,
} = storeToRefs(pluginsStore)

const navigation = ref<HTMLElement | null>(null)
const pluginFilter = ref('')
const expandedPluginIds = shallowRef(new Set<string>())
const transitionDirection = ref<'forward' | 'back'>('forward')

const visibleScope = computed<NavigationScope>(() => props.collapsed ? 'root' : props.scope)
const transitionName = computed(() => transitionDirection.value === 'back'
  ? 'sidebar-navigation-pop'
  : 'sidebar-navigation-push')
const activePluginId = computed(() => route.name === 'plugin-detail'
  ? String(route.params.id ?? '')
  : '')
const centerSelectedKeys = computed(() => {
  if (isPluginCenterRoute(route.name)) {
    return [`page:${String(route.name)}`]
  }

  if (!activePluginId.value || readPluginDetailPanel(route.query) !== 'management-ui') {
    return activePluginId.value ? [`plugin-page:${activePluginId.value}:overview`] : []
  }

  const page = readPluginManagementPage(route.query)
  return page ? [`plugin-page:${activePluginId.value}:management:${page}`] : []
})
const activePlugin = computed(() => current.value?.id === activePluginId.value ? current.value : null)
const activePluginSummary = computed(() => sortedItems.value.find(plugin => plugin.id === activePluginId.value))
const activePluginName = computed(() => (
  activePlugin.value?.name?.trim()
  || activePluginSummary.value?.name?.trim()
  || pluginsStore.getPluginDisplayName(activePluginId.value)
))
const openPluginTargets = computed(() => {
  const seen = new Set<string>()
  return props.openPluginTargets.filter((target) => {
    if (!target.pluginId || !target.fullPath || seen.has(target.pluginId)) return false
    seen.add(target.pluginId)
    return true
  })
})
const openPluginTargetById = computed(() => new Map(
  openPluginTargets.value.map(target => [target.pluginId, target.fullPath]),
))
const navigationPlugins = computed<SidebarPluginItem[]>(() => {
  const items: SidebarPluginItem[] = sortedItems.value.map(plugin => ({
    icon: plugin.icon,
    id: plugin.id,
    name: plugin.name,
    state: plugin.state,
    version: plugin.version,
  }))

  if (activePluginId.value && !items.some(plugin => plugin.id === activePluginId.value)) {
    items.push({
      icon: activePlugin.value?.icon,
      id: activePluginId.value,
      name: activePluginName.value,
      state: activePlugin.value?.state,
      version: activePlugin.value?.version,
    })
  }

  return items.sort((left, right) => left.id.localeCompare(right.id))
})
const showPluginFilter = computed(() => sortedItems.value.length >= 8)
const filteredPlugins = computed(() => {
  const query = pluginFilter.value.trim().toLocaleLowerCase()
  if (!query) return navigationPlugins.value

  return navigationPlugins.value.filter((plugin) => (
    plugin.id === activePluginId.value
    || plugin.id.toLocaleLowerCase().includes(query)
    || plugin.name.toLocaleLowerCase().includes(query)
  ))
})
const pluginNavigationEntries = computed<SidebarPluginNavigationEntry[]>(() => (
  filteredPlugins.value.flatMap((plugin) => {
    const entries: SidebarPluginNavigationEntry[] = [{
      key: `plugin:${plugin.id}`,
      kind: 'resource',
      plugin,
    }]
    if (!isPluginContentVisible(plugin.id)) {
      return entries
    }

    entries.push({
      key: `plugin-page:${plugin.id}:overview`,
      kind: 'overview',
      plugin,
    })
    entries.push(...getPluginManagementPages(plugin.id).map(page => ({
      key: `plugin-page:${plugin.id}:management:${page.id}`,
      kind: 'management' as const,
      page,
      plugin,
    })))
    if (isPluginDetailUnavailable(plugin.id)) {
      entries.push({
        key: `plugin-page:${plugin.id}:retry`,
        kind: 'retry',
        plugin,
      })
    }
    return entries
  })
))

watch(
  () => props.scope,
  (scope) => {
    if (scope === 'plugin-center') {
      void pluginsStore.ensureList().catch(() => undefined)
    }
  },
  { immediate: true },
)

watch(
  activePluginId,
  (pluginId) => {
    if (pluginId) {
      expandPlugin(pluginId)
    }
  },
  { immediate: true },
)

function staticPageTarget(page: (typeof pluginCenterPages)[number]) {
  return route.name === page.name ? route.fullPath : page.path
}

function navigateStaticPage(page: (typeof pluginCenterPages)[number]) {
  emit('scopeChange', 'plugin-center')
  emit('navigate', staticPageTarget(page))
}

function enterPluginCenter() {
  transitionDirection.value = 'forward'
  emit('scopeChange', 'plugin-center')
  const navigates = !isPluginWorkspaceRoute(route.name)
  if (navigates) emit('navigate', '/plugins')
  void focusAfterScopeChange('[data-sidebar-scope-back="plugin-center"]', navigates)
}

function enterPlugin(pluginId: string) {
  expandPlugin(pluginId)
  if (pluginId === activePluginId.value) return
  emit('navigate', openPluginTargetById.value.get(pluginId) ?? buildPluginDetailLocation(pluginId))
}

function openPluginOverview(pluginId: string) {
  emit('navigate', buildPluginDetailLocation(pluginId))
}

function openManagementPage(pluginId: string, pageId: string) {
  emit('navigate', buildPluginDetailLocation(pluginId, {
    panel: 'management-ui',
    managementPage: pageId,
  }))
}

function isPluginExpanded(pluginId: string) {
  return expandedPluginIds.value.has(pluginId)
}

function expandPlugin(pluginId: string) {
  if (!expandedPluginIds.value.has(pluginId)) {
    expandedPluginIds.value = new Set([...expandedPluginIds.value, pluginId])
  }
  void pluginsStore.ensureDetail(pluginId).catch(() => undefined)
}

function togglePluginExpansion(pluginId: string) {
  if (expandedPluginIds.value.has(pluginId)) {
    const nextExpandedPluginIds = new Set(expandedPluginIds.value)
    nextExpandedPluginIds.delete(pluginId)
    expandedPluginIds.value = nextExpandedPluginIds
    return
  }

  expandPlugin(pluginId)
}

function getPluginDetail(pluginId: string) {
  return detailsByPluginId.value[pluginId] ?? (
    current.value?.id === pluginId ? current.value : null
  )
}

function getPluginManagementPages(pluginId: string) {
  return getPluginDetail(pluginId)?.management_ui?.pages ?? []
}

function isPluginDetailPending(pluginId: string) {
  return Boolean(detailLoadingByPluginId.value[pluginId])
}

function isPluginExpansionPending(pluginId: string) {
  return isPluginExpanded(pluginId)
    && isPluginDetailPending(pluginId)
    && !getPluginDetail(pluginId)
}

function isPluginContentVisible(pluginId: string) {
  return isPluginExpanded(pluginId) && !isPluginExpansionPending(pluginId)
}

function isPluginDetailUnavailable(pluginId: string) {
  return Boolean(detailErrorsByPluginId.value[pluginId])
    && !isPluginDetailPending(pluginId)
    && !getPluginDetail(pluginId)
}

function backToRoot() {
  transitionDirection.value = 'back'
  emit('scopeChange', 'root')
  void focusAfterScopeChange('[data-sidebar-entry="plugin-center"]')
}

function openWorkspacePlugin(target: OpenPluginTarget) {
  emit('navigate', target.fullPath)
}

function retryPluginList() {
  void pluginsStore.fetchList().catch(() => undefined)
}

function retryPluginDetail(pluginId: string) {
  void pluginsStore.ensureDetail(pluginId, { refresh: true }).catch(() => undefined)
}

function activatePluginNavigationEntry(entry: SidebarPluginNavigationEntry) {
  if (entry.kind === 'resource') {
    enterPlugin(entry.plugin.id)
  } else if (entry.kind === 'overview') {
    openPluginOverview(entry.plugin.id)
  } else if (entry.kind === 'management') {
    openManagementPage(entry.plugin.id, entry.page.id)
  } else if (entry.kind === 'retry') {
    retryPluginDetail(entry.plugin.id)
  }
}

async function focusAfterScopeChange(selector: string, closesOnMobile = false) {
  if (props.mobile && closesOnMobile) return
  await nextTick()
  navigation.value?.querySelector<HTMLElement>(selector)?.focus()
}

function getPluginSummary(pluginId: string) {
  return navigationPlugins.value.find(plugin => plugin.id === pluginId)
}

function getPluginName(pluginId: string) {
  return getPluginSummary(pluginId)?.name || pluginsStore.getPluginDisplayName(pluginId)
}

function getPluginAriaLabel(plugin: SidebarPluginItem) {
  return plugin.state
    ? `${plugin.name}，${getPluginStateLabel(plugin.state)}`
    : plugin.name
}

function getPluginDisclosureLabel(plugin: SidebarPluginItem) {
  if (isPluginExpansionPending(plugin.id)) {
    return t('plugins.navigation.loadingPluginPages', { name: plugin.name })
  }

  return isPluginExpanded(plugin.id)
    ? t('plugins.navigation.collapsePluginPages', { name: plugin.name })
    : t('plugins.navigation.expandPluginPages', { name: plugin.name })
}
</script>

<template>
  <div
    ref="navigation"
    class="sidebar-navigation"
    :data-mobile="mobile ? 'true' : undefined"
    :data-scope="visibleScope"
  >
    <Transition :name="transitionName">
      <div :key="visibleScope" class="sidebar-navigation__stage">
        <a-menu
          v-if="visibleScope === 'root'"
          mode="inline"
          :open-keys="openKeys"
          :selected-keys="selectedKeys"
          @openChange="emit('openChange', $event)"
        >
          <template v-for="item in menuItems" :key="item.key">
            <a-sub-menu v-if="item.key === pluginCenterTabName && collapsed" :key="item.key">
              <template #title>
                <span class="admin-layout__menu-label" data-sidebar-entry="plugin-center">
                  <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
                  <span>{{ item.title }}</span>
                </span>
              </template>
              <a-menu-item
                v-for="page in pluginCenterPages"
                :key="`page:${page.name}`"
                :data-sidebar-page="page.name"
                @click="navigateStaticPage(page)"
              >
                <span class="admin-layout__menu-label">
                  <component :is="resolveMenuIcon(page.icon)" class="admin-layout__menu-icon" />
                  <span>{{ t(page.titleKey) }}</span>
                </span>
              </a-menu-item>
              <a-menu-item-group v-if="openPluginTargets.length" key="open-plugins">
                <template #title>{{ t('plugins.navigation.groups.openPlugins') }}</template>
                <a-menu-item
                  v-for="target in openPluginTargets"
                  :key="`open-plugin:${target.pluginId}`"
                  :data-sidebar-open-plugin-id="target.pluginId"
                  @click="openWorkspacePlugin(target)"
                >
                  <span class="admin-layout__menu-label sidebar-navigation__plugin-label">
                    <PluginIcon
                      class="sidebar-navigation__plugin-icon"
                      :plugin-id="target.pluginId"
                      :icon="getPluginSummary(target.pluginId)?.icon"
                      :version="getPluginSummary(target.pluginId)?.version"
                    />
                    <span class="sidebar-navigation__plugin-copy" :title="getPluginName(target.pluginId)">
                      {{ getPluginName(target.pluginId) }}
                    </span>
                  </span>
                </a-menu-item>
              </a-menu-item-group>
            </a-sub-menu>

            <a-sub-menu v-else-if="item.children?.length" :key="item.key">
              <template #title>
                <span class="admin-layout__menu-label">
                  <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
                  <span>{{ item.title }}</span>
                </span>
              </template>

              <a-menu-item
                v-for="child in item.children"
                :key="child.key"
                @click="emit('navigate', child.path)"
              >
                <span class="admin-layout__menu-label">
                  <component :is="resolveMenuIcon(child.icon)" v-if="resolveMenuIcon(child.icon)" class="admin-layout__menu-icon" />
                  <span>{{ child.title }}</span>
                </span>
              </a-menu-item>
            </a-sub-menu>

            <a-menu-item
              v-else
              :key="item.key"
              :data-sidebar-entry="item.key === pluginCenterTabName ? 'plugin-center' : undefined"
              @click="item.key === pluginCenterTabName ? enterPluginCenter() : emit('navigate', item.path)"
            >
              <span class="admin-layout__menu-label sidebar-navigation__root-label">
                <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
                <span>{{ item.title }}</span>
                <RightOutlined v-if="item.key === pluginCenterTabName && !collapsed" class="sidebar-navigation__chevron" aria-hidden="true" />
              </span>
            </a-menu-item>
          </template>
        </a-menu>

        <section v-else class="sidebar-navigation__scope" data-testid="plugin-center-sidebar-navigation">
          <button
            type="button"
            class="sidebar-navigation__back"
            data-sidebar-scope-back="plugin-center"
            @click="backToRoot"
          >
            <ArrowLeftOutlined aria-hidden="true" />
            <span>{{ t('routes.pluginCenter') }}</span>
          </button>

          <a-menu mode="inline" :selected-keys="centerSelectedKeys">
            <a-menu-item-group v-for="group in pluginCenterPageGroups" :key="group.key">
              <template #title>{{ t(group.titleKey) }}</template>
              <a-menu-item
                v-for="page in pluginCenterPages.filter(item => item.group === group.key)"
                :key="`page:${page.name}`"
                :data-sidebar-page="page.name"
                @click="navigateStaticPage(page)"
              >
                <span class="admin-layout__menu-label">
                  <component :is="resolveMenuIcon(page.icon)" class="admin-layout__menu-icon" />
                  <span>{{ t(page.titleKey) }}</span>
                </span>
              </a-menu-item>
            </a-menu-item-group>

          </a-menu>

          <a-input
            v-if="showPluginFilter"
            v-model:value="pluginFilter"
            allow-clear
            class="sidebar-navigation__filter"
            :aria-label="t('plugins.navigation.filterLabel')"
            :placeholder="t('plugins.navigation.filterPlaceholder')"
          >
            <template #prefix><SearchOutlined aria-hidden="true" /></template>
          </a-input>

          <a-menu mode="inline" :selected-keys="centerSelectedKeys">
            <a-menu-item-group key="installed-plugins">
              <template #title>{{ t('plugins.navigation.groups.installed') }}</template>
              <a-menu-item
                v-for="entry in pluginNavigationEntries"
                :key="entry.key"
                :aria-busy="entry.kind === 'resource' && isPluginExpansionPending(entry.plugin.id) ? 'true' : undefined"
                :aria-expanded="entry.kind === 'resource' ? isPluginContentVisible(entry.plugin.id) : undefined"
                :aria-label="entry.kind === 'resource' ? getPluginAriaLabel(entry.plugin) : undefined"
                :class="[
                  entry.kind === 'resource' ? 'sidebar-navigation__plugin-resource' : 'sidebar-navigation__plugin-child',
                  {
                    'sidebar-navigation__plugin-resource--active': entry.kind === 'resource' && entry.plugin.id === activePluginId,
                    'sidebar-navigation__plugin-resource--expanded': entry.kind === 'resource' && isPluginContentVisible(entry.plugin.id),
                    'sidebar-navigation__plugin-retry': entry.kind === 'retry',
                  },
                ]"
                :data-sidebar-management-page="entry.kind === 'management' ? entry.page.id : undefined"
                :data-sidebar-plugin-id="entry.kind === 'resource' ? entry.plugin.id : undefined"
                :data-sidebar-plugin-overview="entry.kind === 'overview' ? entry.plugin.id : undefined"
                :data-sidebar-plugin-page-owner="entry.kind === 'resource' ? undefined : entry.plugin.id"
                :data-sidebar-plugin-retry="entry.kind === 'retry' ? entry.plugin.id : undefined"
                @click="activatePluginNavigationEntry(entry)"
              >
                <template v-if="entry.kind === 'resource'">
                  <span class="admin-layout__menu-label sidebar-navigation__plugin-label">
                    <PluginIcon
                      class="sidebar-navigation__plugin-icon"
                      :plugin-id="entry.plugin.id"
                      :icon="entry.plugin.icon"
                      :version="entry.plugin.version"
                    />
                    <span class="sidebar-navigation__plugin-copy" :title="entry.plugin.name">{{ entry.plugin.name }}</span>
                    <span
                      v-if="entry.plugin.state"
                      class="sidebar-navigation__state"
                      :data-state="entry.plugin.state"
                      :title="getPluginStateLabel(entry.plugin.state)"
                      aria-hidden="true"
                    />
                    <button
                      type="button"
                      class="sidebar-navigation__disclosure"
                      :aria-busy="isPluginExpansionPending(entry.plugin.id) ? 'true' : undefined"
                      :aria-expanded="isPluginContentVisible(entry.plugin.id)"
                      :aria-label="getPluginDisclosureLabel(entry.plugin)"
                      :data-sidebar-plugin-disclosure="entry.plugin.id"
                      :title="getPluginDisclosureLabel(entry.plugin)"
                      @click.stop="togglePluginExpansion(entry.plugin.id)"
                    >
                      <LoadingOutlined v-if="isPluginExpansionPending(entry.plugin.id)" class="sidebar-navigation__loading-icon" aria-hidden="true" />
                      <DownOutlined v-else-if="isPluginContentVisible(entry.plugin.id)" aria-hidden="true" />
                      <RightOutlined v-else aria-hidden="true" />
                    </button>
                  </span>
                </template>
                <span v-else-if="entry.kind === 'overview'" class="admin-layout__menu-label">
                  <component :is="resolveMenuIcon('plugins')" class="admin-layout__menu-icon" />
                  <span>{{ t('plugins.panels.overview') }}</span>
                </span>
                <span v-else-if="entry.kind === 'management'" class="admin-layout__menu-label">
                  <component :is="resolveMenuIcon('plugin-settings')" class="admin-layout__menu-icon" />
                  <span :title="entry.page.label">{{ entry.page.label }}</span>
                </span>
                <span v-else class="admin-layout__menu-label">
                  <ReloadOutlined aria-hidden="true" />
                  <span>{{ t('plugins.navigation.detailUnavailable') }} · {{ t('plugins.navigation.retry') }}</span>
                </span>
              </a-menu-item>
            </a-menu-item-group>
          </a-menu>

          <a-skeleton v-if="loading && navigationPlugins.length === 0" class="sidebar-navigation__skeleton" active :title="false" :paragraph="{ rows: 3 }" />
          <div v-else-if="error" class="sidebar-navigation__feedback" role="status">
            <span>{{ t('plugins.navigation.listLoadFailed') }}</span>
            <a-button type="link" size="small" @click="retryPluginList">
              <template #icon><ReloadOutlined /></template>
              {{ t('plugins.navigation.retry') }}
            </a-button>
          </div>
          <div v-else-if="navigationPlugins.length === 0" class="sidebar-navigation__feedback">
            {{ t('plugins.navigation.emptyInstalled') }}
          </div>
          <div v-else-if="filteredPlugins.length === 0" class="sidebar-navigation__feedback">
            {{ t('plugins.navigation.emptyFilter') }}
          </div>
        </section>
      </div>
    </Transition>
  </div>
</template>

<style scoped lang="scss">
.sidebar-navigation {
  display: grid;
  min-width: 0;
  overflow: hidden;
}

.sidebar-navigation__stage {
  grid-area: 1 / 1;
  min-width: 0;
}

.sidebar-navigation__scope {
  display: grid;
  align-content: start;
  min-width: 0;
}

.sidebar-navigation__back {
  display: flex;
  align-items: center;
  gap: 9px;
  width: calc(100% - 8px);
  min-height: 38px;
  margin: 0 4px 8px;
  padding: 6px 10px;
  overflow: hidden;
  border: 0;
  border-radius: 8px;
  background: var(--sider-menu-active-bg);
  color: var(--sider-brand-text);
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  text-align: left;
}

.sidebar-navigation__back:hover {
  background: var(--sider-menu-hover-bg);
  color: var(--sider-menu-active);
}

.sidebar-navigation__back:focus-visible {
  outline: 2px solid var(--chrome-muted);
  outline-offset: 2px;
}

.sidebar-navigation__back > span:last-child {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-navigation__root-label,
.sidebar-navigation__plugin-label {
  min-width: 0;
}

.sidebar-navigation__chevron {
  flex: 0 0 auto;
  margin-inline-start: auto;
  color: var(--chrome-muted);
  font-size: 11px;
}

.sidebar-navigation__disclosure {
  display: inline-grid;
  width: 28px;
  height: 28px;
  flex: 0 0 auto;
  place-items: center;
  margin: -4px -8px -4px 0;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--chrome-muted);
  cursor: pointer;
  font-size: 11px;
}

.sidebar-navigation__disclosure:hover {
  background: var(--sider-menu-hover-bg);
  color: var(--sider-menu-active);
}

.sidebar-navigation__disclosure:focus-visible {
  outline: 2px solid var(--chrome-muted);
  outline-offset: -2px;
}

.sidebar-navigation__plugin-copy {
  width: 0;
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-navigation__state {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border: 1px solid currentColor;
  border-radius: 50%;
  background: transparent;
  color: var(--chrome-muted);
}

.sidebar-navigation__state[data-state='enabled'],
.sidebar-navigation__state[data-state='running'] {
  border-color: var(--success);
  background: var(--success);
  color: var(--success);
}

.sidebar-navigation__state[data-state='failed'],
.sidebar-navigation__state[data-state='invalid'] {
  border-color: var(--danger);
  background: var(--danger);
  color: var(--danger);
}

.sidebar-navigation__plugin-icon {
  width: 20px;
  height: 20px;
}

.sidebar-navigation__plugin-icon :deep(.raylea-mark) {
  width: 18px;
  height: 18px;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-resource--active.ant-menu-item) {
  background: var(--sider-menu-hover-bg);
  color: var(--sider-menu-active);
  font-weight: 600;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-resource--expanded.ant-menu-item) {
  font-weight: 600;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-child.ant-menu-item) {
  padding-inline-start: 40px !important;
  animation: sidebar-navigation-reveal 160ms cubic-bezier(0.2, 0.8, 0.2, 1) both;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-child .admin-layout__menu-icon) {
  font-size: 13px;
}

.sidebar-navigation__loading-icon {
  animation: sidebar-navigation-spin 900ms linear infinite;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-retry.ant-menu-item) {
  height: auto;
  min-height: 36px;
  line-height: 1.35;
  white-space: normal;
}

.sidebar-navigation__filter {
  width: calc(100% - 8px);
  margin: 8px 4px 0;
  border-radius: 8px;
}

.sidebar-navigation__skeleton,
.sidebar-navigation__feedback {
  width: calc(100% - 8px);
  margin: 4px;
  padding: 10px 12px;
}

.sidebar-navigation__feedback {
  display: grid;
  gap: 4px;
  color: var(--chrome-muted);
  font-size: 12px;
  line-height: 1.5;
}

.sidebar-navigation__feedback :deep(.ant-btn) {
  justify-self: start;
  height: auto;
  padding: 0;
}

.sidebar-navigation :deep(.ant-menu-item-group-title) {
  padding: 12px 16px 4px;
  color: var(--chrome-muted);
  font-size: 11px;
  line-height: 1.4;
}

.sidebar-navigation :deep(.ant-menu-item) {
  overflow: hidden;
}

.sidebar-navigation :deep(.ant-menu-item .ant-menu-title-content) {
  min-width: 0;
}

@keyframes sidebar-navigation-reveal {
  from {
    opacity: 0;
    transform: translateY(-4px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes sidebar-navigation-spin {
  to {
    transform: rotate(360deg);
  }
}

.sidebar-navigation-push-enter-active,
.sidebar-navigation-push-leave-active,
.sidebar-navigation-pop-enter-active,
.sidebar-navigation-pop-leave-active {
  transition: opacity 160ms cubic-bezier(0.2, 0.8, 0.2, 1), transform 160ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.sidebar-navigation-push-enter-from,
.sidebar-navigation-pop-leave-to {
  opacity: 0;
  transform: translateX(12px);
}

.sidebar-navigation-push-leave-to,
.sidebar-navigation-pop-enter-from {
  opacity: 0;
  transform: translateX(-12px);
}

@media (max-width: 1024px), (pointer: coarse) {
  .sidebar-navigation__back {
    min-height: 44px;
  }

  .sidebar-navigation__disclosure {
    width: 44px;
    height: 44px;
    margin: 0 -12px 0 0;
  }

  .sidebar-navigation :deep(.sidebar-navigation__plugin-resource.ant-menu-item) {
    height: 44px;
    line-height: 44px;
  }
}

@media (prefers-reduced-motion: reduce), (forced-colors: active) {
  .sidebar-navigation-push-enter-active,
  .sidebar-navigation-push-leave-active,
  .sidebar-navigation-pop-enter-active,
  .sidebar-navigation-pop-leave-active {
    transition: none;
  }

  .sidebar-navigation :deep(.sidebar-navigation__plugin-child.ant-menu-item) {
    animation: none;
  }

  .sidebar-navigation__loading-icon {
    animation: none;
  }

  .sidebar-navigation-push-enter-from,
  .sidebar-navigation-push-leave-to,
  .sidebar-navigation-pop-enter-from,
  .sidebar-navigation-pop-leave-to {
    transform: none;
  }
}
</style>
