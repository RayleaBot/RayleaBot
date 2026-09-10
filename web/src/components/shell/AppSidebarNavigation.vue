<script setup lang="ts">
import { ArrowLeftIcon, ChevronDownIcon, ChevronRightIcon, LoaderCircleIcon, RotateCwIcon } from '@lucide/vue'
import AppDropdown from '@/components/AppDropdown.vue'
import AppDropdownItem from '@/components/AppDropdownItem.vue'
import AppInput from '@/components/AppInput.vue'
import AppButton from '@/components/AppButton.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
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
import { usePluginCollection } from '@/lib/use-plugin-collection'
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
} = storeToRefs(pluginsStore)

const pluginCollection = usePluginCollection()
const { items: sortedItems, error, loading, total, nextCursor, loadingMore } = pluginCollection
const navigation = ref<HTMLElement | null>(null)
const pluginFilter = ref('')
const expandedPluginIds = shallowRef(new Set<string>())
const visitedPluginIds = shallowRef(new Set<string>())
watch(() => Object.keys(detailsByPluginId.value), ids => {
  visitedPluginIds.value = new Set([...visitedPluginIds.value, ...ids])
}, { immediate: true })
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
const activePluginSummary = computed(() => pluginsStore.knownItems.find(plugin => plugin.id === activePluginId.value))
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
  const visited = pluginsStore.knownItems.filter(plugin => visitedPluginIds.value.has(plugin.id))
  const remembered = new Map([...sortedItems.value, ...visited, ...Object.values(detailsByPluginId.value)].map(plugin => [plugin.id, plugin]))
  const items: SidebarPluginItem[] = [...remembered.values()].map(plugin => ({
    icon: plugin.icon,
    id: plugin.id,
    name: plugin.name,
    state: plugin.state,
    version: plugin.version,
  }))

  for (const target of openPluginTargets.value) {
    if (!items.some(plugin => plugin.id === target.pluginId)) {
      const known = pluginsStore.knownItems.find(plugin => plugin.id === target.pluginId)
      items.push(known ?? { id: target.pluginId, name: pluginsStore.getPluginDisplayName(target.pluginId) })
    }
  }
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
const showPluginFilter = computed(() => total.value >= 8 || Boolean(pluginFilter.value))
const filteredPlugins = computed(() => {
  if (!pluginFilter.value.trim()) return navigationPlugins.value
  const matchingIds = new Set(sortedItems.value.map(plugin => plugin.id))
  return navigationPlugins.value.filter(plugin => plugin.id === activePluginId.value || matchingIds.has(plugin.id))
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
      void pluginCollection.load({ query: pluginFilter.value }).catch(() => undefined)
    }
  },
  { immediate: true },
)

watch(pluginFilter, (query, _, cleanup) => {
  pluginCollection.cancel()
  const timer = setTimeout(() => { void pluginCollection.load({ query }).catch(() => undefined) }, 250)
  cleanup(() => clearTimeout(timer))
})

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
  void pluginCollection.load({ query: pluginFilter.value }).catch(() => undefined)
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
function toggleRootGroup(key: string) {
  emit('openChange', props.openKeys.includes(key) ? props.openKeys.filter(item => item !== key) : [...props.openKeys, key])
}
</script>

<template>
  <div ref="navigation" class="sidebar-navigation" :data-mobile="mobile ? 'true' : undefined" :data-collapsed="collapsed" :data-scope="visibleScope">
    <Transition :name="transitionName">
      <div :key="visibleScope" class="sidebar-navigation__stage">
        <div v-if="visibleScope === 'root'" class="sidebar-navigation__root">
          <template v-for="item in menuItems" :key="item.key">
            <AppDropdown v-if="collapsed && (item.children?.length || item.key === pluginCenterTabName)" side="right" align="start">
              <button type="button" class="sidebar-navigation__item sidebar-navigation__collapsed-item" data-nav-item :aria-label="item.title" :title="item.title" :data-sidebar-entry="item.key === pluginCenterTabName ? 'plugin-center' : undefined">
                <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
              </button>
              <template #content>
                <template v-if="item.key === pluginCenterTabName">
                  <AppDropdownItem v-for="page in pluginCenterPages" :key="page.name" :data-sidebar-page="page.name" @select="navigateStaticPage(page)"><component :is="resolveMenuIcon(page.icon)" />{{ t(page.titleKey) }}</AppDropdownItem>
                  <div v-if="openPluginTargets.length" class="app-menu-label">{{ t('plugins.navigation.groups.openPlugins') }}</div>
                  <AppDropdownItem v-for="target in openPluginTargets" :key="target.pluginId" :data-sidebar-open-plugin-id="target.pluginId" @select="openWorkspacePlugin(target)">
                    <PluginIcon :refresh-key="pluginsStore.iconRevision" class="sidebar-navigation__plugin-icon" :plugin-id="target.pluginId" :icon="getPluginSummary(target.pluginId)?.icon" :version="getPluginSummary(target.pluginId)?.version" />{{ getPluginName(target.pluginId) }}
                  </AppDropdownItem>
                </template>
                <AppDropdownItem v-for="child in item.children" v-else :key="child.key" @select="emit('navigate', child.path)"><component :is="resolveMenuIcon(child.icon)" v-if="resolveMenuIcon(child.icon)" />{{ child.title }}</AppDropdownItem>
              </template>
            </AppDropdown>
            <section v-else-if="item.children?.length" class="sidebar-navigation__group">
              <button type="button" class="sidebar-navigation__group-heading" data-nav-item :aria-expanded="openKeys.includes(item.key)" @click="toggleRootGroup(item.key)">{{ item.title }}<ChevronDownIcon :class="{ 'is-collapsed': !openKeys.includes(item.key) }" :size="13" /></button>
              <div v-if="openKeys.includes(item.key)">
                <button v-for="child in item.children" :key="child.key" type="button" class="sidebar-navigation__item" data-nav-item :aria-current="selectedKeys.includes(child.key) ? 'page' : undefined" @click="emit('navigate', child.path)">
                  <span class="admin-layout__menu-label"><component :is="resolveMenuIcon(child.icon)" v-if="resolveMenuIcon(child.icon)" class="admin-layout__menu-icon" /><span>{{ child.title }}</span></span>
                </button>
              </div>
            </section>
            <button v-else type="button" class="sidebar-navigation__item" :class="{ 'sidebar-navigation__collapsed-item': collapsed }" data-nav-item :aria-label="item.title" :title="collapsed ? item.title : undefined" :aria-current="selectedKeys.includes(item.key) ? 'page' : undefined" :data-sidebar-entry="item.key === pluginCenterTabName ? 'plugin-center' : undefined" @click="item.key === pluginCenterTabName ? enterPluginCenter() : emit('navigate', item.path)">
              <span class="admin-layout__menu-label sidebar-navigation__root-label"><component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" /><span v-if="!collapsed">{{ item.title }}</span><ChevronRightIcon v-if="item.key === pluginCenterTabName && !collapsed" class="sidebar-navigation__chevron" aria-hidden="true" /></span>
            </button>
          </template>
        </div>
        <section v-else class="sidebar-navigation__scope" data-testid="plugin-center-sidebar-navigation">
          <button type="button" class="sidebar-navigation__back" data-sidebar-scope-back="plugin-center" data-nav-item @click="backToRoot"><ArrowLeftIcon aria-hidden="true" /><span>{{ t('routes.pluginCenter') }}</span></button>
          <section v-for="group in pluginCenterPageGroups" :key="group.key" class="sidebar-navigation__group">
            <h3 class="sidebar-navigation__group-title">{{ t(group.titleKey) }}</h3>
            <button v-for="page in pluginCenterPages.filter(item => item.group === group.key)" :key="page.name" type="button" class="sidebar-navigation__item" data-nav-item :aria-current="centerSelectedKeys.includes('page:' + page.name) ? 'page' : undefined" :data-sidebar-page="page.name" @click="navigateStaticPage(page)">
              <span class="admin-layout__menu-label"><component :is="resolveMenuIcon(page.icon)" class="admin-layout__menu-icon" /><span>{{ t(page.titleKey) }}</span></span>
            </button>
          </section>
          <div v-if="showPluginFilter" class="sidebar-navigation__filter"><AppInput v-model="pluginFilter" allow-clear :aria-label="t('plugins.navigation.filterLabel')" :placeholder="t('plugins.navigation.filterPlaceholder')" /></div>
          <section class="sidebar-navigation__group">
            <h3 class="sidebar-navigation__group-title">{{ t('plugins.navigation.groups.installed') }}</h3>
            <div v-for="entry in pluginNavigationEntries" :key="entry.key" class="sidebar-navigation__entry" :class="{ 'sidebar-navigation__entry--resource': entry.kind === 'resource', 'sidebar-navigation__entry--active': entry.kind === 'resource' && entry.plugin.id === activePluginId }">
              <button type="button" class="sidebar-navigation__item" data-nav-item
                :aria-expanded="entry.kind === 'resource' ? isPluginContentVisible(entry.plugin.id) : undefined"
                :aria-busy="entry.kind === 'resource' && isPluginExpansionPending(entry.plugin.id) ? 'true' : undefined"
                :aria-label="entry.kind === 'resource' ? getPluginAriaLabel(entry.plugin) : undefined"
                :aria-current="centerSelectedKeys.includes(entry.key) ? 'page' : undefined"
                :class="[entry.kind === 'resource' ? 'sidebar-navigation__plugin-resource' : 'sidebar-navigation__plugin-child', { 'sidebar-navigation__plugin-resource--active': entry.kind === 'resource' && entry.plugin.id === activePluginId }]"
                :data-sidebar-management-page="entry.kind === 'management' ? entry.page.id : undefined"
                :data-sidebar-plugin-id="entry.kind === 'resource' ? entry.plugin.id : undefined"
                :data-sidebar-plugin-overview="entry.kind === 'overview' ? entry.plugin.id : undefined"
                :data-sidebar-plugin-page-owner="entry.kind === 'resource' ? undefined : entry.plugin.id"
                :data-sidebar-plugin-retry="entry.kind === 'retry' ? entry.plugin.id : undefined"
                @click="activatePluginNavigationEntry(entry)">
                <span v-if="entry.kind === 'resource'" class="admin-layout__menu-label sidebar-navigation__plugin-label">
                  <PluginIcon :refresh-key="pluginsStore.iconRevision" class="sidebar-navigation__plugin-icon" :plugin-id="entry.plugin.id" :icon="entry.plugin.icon" :version="entry.plugin.version" />
                  <span class="sidebar-navigation__plugin-copy" :title="entry.plugin.name">{{ entry.plugin.name }}</span>
                  <span v-if="entry.plugin.state" class="sidebar-navigation__state" :data-state="entry.plugin.state" :title="getPluginStateLabel(entry.plugin.state)" aria-hidden="true" />
                </span>
                <span v-else-if="entry.kind === 'overview'" class="admin-layout__menu-label"><component :is="resolveMenuIcon('plugins')" class="admin-layout__menu-icon" /><span>{{ t('plugins.panels.overview') }}</span></span>
                <span v-else-if="entry.kind === 'management'" class="admin-layout__menu-label"><component :is="resolveMenuIcon('plugin-settings')" class="admin-layout__menu-icon" /><span :title="entry.page.label">{{ entry.page.label }}</span></span>
                <span v-else class="admin-layout__menu-label"><RotateCwIcon aria-hidden="true" /><span>{{ t('plugins.navigation.detailUnavailable') }} · {{ t('plugins.navigation.retry') }}</span></span>
              </button>
              <button v-if="entry.kind === 'resource'" type="button" class="sidebar-navigation__disclosure" :aria-busy="isPluginExpansionPending(entry.plugin.id) ? 'true' : undefined" :aria-expanded="isPluginContentVisible(entry.plugin.id)" :aria-label="getPluginDisclosureLabel(entry.plugin)" :data-sidebar-plugin-disclosure="entry.plugin.id" :title="getPluginDisclosureLabel(entry.plugin)" @click="togglePluginExpansion(entry.plugin.id)">
                <LoaderCircleIcon v-if="isPluginExpansionPending(entry.plugin.id)" class="sidebar-navigation__loading-icon" aria-hidden="true" /><ChevronDownIcon v-else-if="isPluginContentVisible(entry.plugin.id)" aria-hidden="true" /><ChevronRightIcon v-else aria-hidden="true" />
              </button>
            </div>
          </section>
          <AppButton v-if="nextCursor" :loading="loading || loadingMore" :disabled="loading || loadingMore" @click="pluginCollection.loadMore().catch(() => undefined)">{{ t('plugins.store.loadMore') }}</AppButton>
          <AppSkeleton v-if="loading && navigationPlugins.length === 0" class="sidebar-navigation__skeleton" :rows="3" />
          <div v-else-if="error" class="sidebar-navigation__feedback" role="status"><span>{{ t('plugins.navigation.listLoadFailed') }}</span><AppButton variant="link" size="sm" @click="retryPluginList"><RotateCwIcon />{{ t('plugins.navigation.retry') }}</AppButton></div>
          <div v-else-if="navigationPlugins.length === 0" class="sidebar-navigation__feedback">{{ t('plugins.navigation.emptyInstalled') }}</div>
          <div v-else-if="filteredPlugins.length === 0" class="sidebar-navigation__feedback">{{ t('plugins.navigation.emptyFilter') }}</div>
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
  outline-offset: var(--focus-outline-offset);
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
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  place-items: center;
  margin: 0;
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

.sidebar-navigation :deep(.sidebar-navigation__plugin-resource--active) {
  background: var(--sider-menu-hover-bg);
  color: var(--sider-menu-active);
  font-weight: 600;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-resource--expanded) {
  font-weight: 600;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-child) {
  padding-inline-start: 40px !important;
  animation: sidebar-navigation-reveal 160ms cubic-bezier(0.2, 0.8, 0.2, 1) both;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-child .admin-layout__menu-icon) {
  font-size: 13px;
}

.sidebar-navigation__loading-icon {
  animation: sidebar-navigation-spin 900ms linear infinite;
}

.sidebar-navigation :deep(.sidebar-navigation__plugin-retry) {
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

.sidebar-navigation__feedback :deep(.app-button) {
  justify-self: start;
  height: auto;
  padding: 0;
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

  .sidebar-navigation :deep(.sidebar-navigation__plugin-resource) {
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

  .sidebar-navigation :deep(.sidebar-navigation__plugin-child) {
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

.sidebar-navigation__group { margin-bottom: 10px; }
.sidebar-navigation__group-title, .sidebar-navigation__group-heading { margin: 0; padding: 12px 12px 6px; color: var(--chrome-muted); font-size: 12px; font-weight: 500; line-height: 1.4; }
.sidebar-navigation__group-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; width: 100%; min-height: 36px; cursor: pointer; }
.sidebar-navigation__group-heading .is-collapsed { rotate: -90deg; }
.sidebar-navigation__item { display: flex; align-items: center; width: calc(100% - 8px); min-height: 40px; margin: 2px 4px; padding: 8px 10px; border-radius: 8px; color: var(--sider-menu-text); font-size: 14px; line-height: 1.4; text-align: left; cursor: pointer; }
.sidebar-navigation__item:hover { background: var(--sider-menu-hover-bg); color: var(--sider-menu-active); }
.sidebar-navigation__item[aria-current=page] { background: var(--sider-menu-active-bg); color: var(--sider-menu-active); font-weight: 600; }
.sidebar-navigation__item:focus-visible, .sidebar-navigation__group-heading:focus-visible { outline: 2px solid var(--chrome-muted); outline-offset: -2px; }
.sidebar-navigation__entry { display: flex; align-items: center; min-width: 0; }
.sidebar-navigation__entry > .sidebar-navigation__item { flex: 1; min-width: 0; }
.sidebar-navigation__entry--active { background: var(--sider-menu-hover-bg); border-radius: 8px; }
.sidebar-navigation__entry--resource .sidebar-navigation__item { margin-right: 0; padding-right: 4px; }
.sidebar-navigation__item .admin-layout__menu-label > span { overflow: hidden; text-overflow: ellipsis; }
.sidebar-navigation__item.sidebar-navigation__plugin-child { padding-inline-start: 38px; }
.sidebar-navigation__disclosure > svg, .sidebar-navigation__chevron { width: 16px; height: 16px; }
.sidebar-navigation__back > svg { width: 18px; height: 18px; }
.sidebar-navigation__item.sidebar-navigation__collapsed-item { width: 40px; height: 40px; justify-content: center; margin: 4px 0; padding: 10px; }
.sidebar-navigation[data-collapsed=true] .admin-layout__menu-label { justify-content: center; }
.sidebar-navigation__filter :deep(.app-input) { background: var(--sider-menu-hover-bg); color: var(--sider-menu-text); border-color: var(--sider-brand-border); font-size: 12px; }
@media (max-width: 991px), (pointer: coarse) { .sidebar-navigation__item, .sidebar-navigation__group-heading { min-height: 44px; } }

</style>
