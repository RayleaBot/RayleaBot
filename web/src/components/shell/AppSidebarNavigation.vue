<script setup lang="ts">
import { ArrowLeftIcon, ChevronDownIcon, ChevronRightIcon, LoaderCircleIcon, RotateCwIcon } from '@lucide/vue'
import AppDropdown from '@/components/AppDropdown.vue'
import AppDropdownItem from '@/components/AppDropdownItem.vue'
import AppInput from '@/components/AppInput.vue'
import AppButton from '@/components/AppButton.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, type RouteLocationRaw } from 'vue-router'

import { resolveMenuIcon } from '@/access/icons'
import type { AppMenuItem } from '@/access/menu'
import {
  isPluginCenterRoute,
  isPluginWorkspaceRoute,
  pluginCenterPageGroups,
  pluginCenterPages,
  pluginCenterMenuKey,
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
import {
  useSidebarPluginNavigation,
  type SidebarPluginNavigationEntry,
} from './useSidebarPluginNavigation'

type NavigationScope = 'root' | 'plugin-center'

const props = withDefaults(defineProps<{
  collapsed?: boolean
  menuItems: AppMenuItem[]
  openKeys: string[]
  scope: NavigationScope
  selectedKeys: string[]
}>(), {
  collapsed: false,
})

const emit = defineEmits<{
  navigate: [target: RouteLocationRaw]
  openChange: [keys: string[]]
  scopeChange: [scope: NavigationScope]
}>()

const route = useRoute()
const pluginsStore = usePluginsStore()
const activePluginId = computed(() => route.name === 'plugin-detail'
  ? String(route.params.id ?? '')
  : '')
const {
  expandPlugin,
  filteredPlugins,
  getPluginAriaLabel,
  getPluginDisclosureLabel,
  isPluginContentVisible,
  isPluginExpansionPending,
  navigationPlugins,
  pluginCollection,
  pluginFilter,
  pluginNavigationEntries,
  retryPluginDetail,
  retryPluginList,
  showPluginFilter,
  togglePluginExpansion,
} = useSidebarPluginNavigation({ activePluginId })
const { error, loading, nextCursor, loadingMore } = pluginCollection
const navigation = ref<HTMLElement | null>(null)

// Each plugin row owns the pages listed under it, so the pages open and close as one block with their plugin.
const pluginNavigationGroups = computed(() => {
  const groups: Array<{ resource: SidebarPluginNavigationEntry; pages: SidebarPluginNavigationEntry[] }> = []
  for (const entry of pluginNavigationEntries.value) {
    if (entry.kind === 'resource') groups.push({ resource: entry, pages: [] })
    else groups.at(-1)?.pages.push(entry)
  }
  return groups
})
const transitionDirection = ref<'forward' | 'back'>('forward')

const visibleScope = computed<NavigationScope>(() => props.collapsed ? 'root' : props.scope)
const transitionName = computed(() => transitionDirection.value === 'back'
  ? 'sidebar-navigation-pop'
  : 'sidebar-navigation-push')
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

watch(
  () => props.scope,
  (scope) => {
    if (scope === 'plugin-center') {
      void pluginCollection.load({ query: pluginFilter.value }).catch(() => undefined)
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
  if (!isPluginWorkspaceRoute(route.name)) emit('navigate', '/plugins')
  void focusAfterScopeChange('[data-sidebar-scope-back="plugin-center"]')
}

function enterPlugin(pluginId: string) {
  expandPlugin(pluginId)
  if (pluginId === activePluginId.value) return
  emit('navigate', buildPluginDetailLocation(pluginId))
}

function backToRoot() {
  transitionDirection.value = 'back'
  emit('scopeChange', 'root')
  void focusAfterScopeChange('[data-sidebar-entry="plugin-center"]')
}

function activatePluginNavigationEntry(entry: SidebarPluginNavigationEntry) {
  if (entry.kind === 'resource') {
    enterPlugin(entry.plugin.id)
  } else if (entry.kind === 'overview') {
    emit('navigate', buildPluginDetailLocation(entry.plugin.id))
  } else if (entry.kind === 'management') {
    emit('navigate', buildPluginDetailLocation(entry.plugin.id, {
      panel: 'management-ui',
      managementPage: entry.page.id,
    }))
  } else if (entry.kind === 'retry') {
    retryPluginDetail(entry.plugin.id)
  }
}

async function focusAfterScopeChange(selector: string) {
  await nextTick()
  navigation.value?.querySelector<HTMLElement>(selector)?.focus()
}

function containsCurrentPage(item: AppMenuItem) {
  return props.selectedKeys.includes(item.key) || Boolean(item.children?.some(child => props.selectedKeys.includes(child.key)))
}

function toggleRootGroup(key: string) {
  emit('openChange', props.openKeys.includes(key) ? props.openKeys.filter(item => item !== key) : [...props.openKeys, key])
}
</script>

<template>
  <div ref="navigation" class="sidebar-navigation" :data-collapsed="collapsed" :data-scope="visibleScope">
    <Transition :name="transitionName">
      <div :key="visibleScope" class="sidebar-navigation__stage">
        <div v-if="visibleScope === 'root'" class="sidebar-navigation__root">
          <template v-for="item in menuItems" :key="item.key">
            <AppDropdown v-if="collapsed && (item.children?.length || item.key === pluginCenterMenuKey)" side="right" align="start">
              <button type="button" class="sidebar-navigation__item sidebar-navigation__collapsed-item" data-nav-item :aria-label="item.title" :title="item.title" :aria-current="containsCurrentPage(item) ? 'true' : undefined" :data-sidebar-entry="item.key === pluginCenterMenuKey ? 'plugin-center' : undefined">
                <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
              </button>
              <template #content>
                <template v-if="item.key === pluginCenterMenuKey">
                  <AppDropdownItem v-for="page in pluginCenterPages" :key="page.name" :data-sidebar-page="page.name" @select="navigateStaticPage(page)"><component :is="resolveMenuIcon(page.icon)" />{{ t(page.titleKey) }}</AppDropdownItem>
                </template>
                <AppDropdownItem v-for="child in item.children" v-else :key="child.key" @select="emit('navigate', child.path)"><component :is="resolveMenuIcon(child.icon)" v-if="resolveMenuIcon(child.icon)" />{{ child.title }}</AppDropdownItem>
              </template>
            </AppDropdown>
            <section v-else-if="item.children?.length" class="sidebar-navigation__group">
              <button type="button" class="sidebar-navigation__group-heading" data-nav-item :aria-expanded="openKeys.includes(item.key)" @click="toggleRootGroup(item.key)">{{ item.title }}<ChevronDownIcon :class="{ 'is-collapsed': !openKeys.includes(item.key) }" :size="16" /></button>
              <Transition name="sidebar-collapse">
                <div v-if="openKeys.includes(item.key)" class="sidebar-collapse">
                  <div class="sidebar-collapse__inner">
                    <button v-for="child in item.children" :key="child.key" type="button" class="sidebar-navigation__item" data-nav-item :aria-current="selectedKeys.includes(child.key) ? 'page' : undefined" @click="emit('navigate', child.path)">
                      <span class="admin-layout__menu-label"><component :is="resolveMenuIcon(child.icon)" v-if="resolveMenuIcon(child.icon)" class="admin-layout__menu-icon" /><span>{{ child.title }}</span></span>
                    </button>
                  </div>
                </div>
              </Transition>
            </section>
            <button v-else type="button" class="sidebar-navigation__item" :class="{ 'sidebar-navigation__collapsed-item': collapsed }" data-nav-item :aria-label="item.title" :title="collapsed ? item.title : undefined" :aria-current="selectedKeys.includes(item.key) ? 'page' : undefined" :data-sidebar-entry="item.key === pluginCenterMenuKey ? 'plugin-center' : undefined" @click="item.key === pluginCenterMenuKey ? enterPluginCenter() : emit('navigate', item.path)">
              <span class="admin-layout__menu-label sidebar-navigation__root-label"><component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" /><span v-if="!collapsed">{{ item.title }}</span><ChevronRightIcon v-if="item.key === pluginCenterMenuKey && !collapsed" class="sidebar-navigation__chevron" aria-hidden="true" /></span>
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
            <div v-for="{ resource, pages } in pluginNavigationGroups" :key="resource.key" class="sidebar-plugin" :data-active="resource.plugin.id === activePluginId || undefined">
              <div class="sidebar-navigation__entry sidebar-navigation__entry--resource">
                <button type="button" class="sidebar-navigation__item sidebar-navigation__plugin-resource" data-nav-item
                  :class="{ 'sidebar-navigation__plugin-resource--active': resource.plugin.id === activePluginId }"
                  :aria-expanded="isPluginContentVisible(resource.plugin.id)"
                  :aria-busy="isPluginExpansionPending(resource.plugin.id) ? 'true' : undefined"
                  :aria-label="getPluginAriaLabel(resource.plugin)"
                  :data-sidebar-plugin-id="resource.plugin.id"
                  @click="activatePluginNavigationEntry(resource)">
                  <span class="admin-layout__menu-label sidebar-navigation__plugin-label">
                    <PluginIcon :refresh-key="pluginsStore.iconRevision" class="sidebar-navigation__plugin-icon" :plugin-id="resource.plugin.id" :icon="resource.plugin.icon" :version="resource.plugin.version" />
                    <span class="sidebar-navigation__plugin-copy" :title="resource.plugin.name">{{ resource.plugin.name }}</span>
                    <span v-if="resource.plugin.state" class="sidebar-navigation__state" :data-state="resource.plugin.state" :title="getPluginStateLabel(resource.plugin.state)" aria-hidden="true" />
                  </span>
                </button>
                <button type="button" class="sidebar-navigation__disclosure" :aria-busy="isPluginExpansionPending(resource.plugin.id) ? 'true' : undefined" :aria-expanded="isPluginContentVisible(resource.plugin.id)" :aria-label="getPluginDisclosureLabel(resource.plugin)" :data-sidebar-plugin-disclosure="resource.plugin.id" :title="getPluginDisclosureLabel(resource.plugin)" @click="togglePluginExpansion(resource.plugin.id)">
                  <LoaderCircleIcon v-if="isPluginExpansionPending(resource.plugin.id)" class="sidebar-navigation__loading-icon" aria-hidden="true" /><ChevronRightIcon v-else class="sidebar-navigation__disclosure-icon" :data-open="isPluginContentVisible(resource.plugin.id) || undefined" aria-hidden="true" />
                </button>
              </div>
              <Transition name="sidebar-collapse">
                <div v-if="pages.length" class="sidebar-collapse" role="group" :aria-label="t('plugins.navigation.pluginPages', { name: resource.plugin.name })">
                  <div class="sidebar-collapse__inner sidebar-plugin__pages">
                    <button v-for="(entry, index) in pages" :key="entry.key" type="button" class="sidebar-navigation__item sidebar-navigation__plugin-child" data-nav-item
                      :style="{ '--reveal-order': index }"
                      :aria-current="centerSelectedKeys.includes(entry.key) ? 'page' : undefined"
                      :data-sidebar-management-page="entry.kind === 'management' ? entry.page.id : undefined"
                      :data-sidebar-plugin-overview="entry.kind === 'overview' ? entry.plugin.id : undefined"
                      :data-sidebar-plugin-page-owner="entry.plugin.id"
                      :data-sidebar-plugin-retry="entry.kind === 'retry' ? entry.plugin.id : undefined"
                      @click="activatePluginNavigationEntry(entry)">
                      <span v-if="entry.kind === 'overview'" class="admin-layout__menu-label"><component :is="resolveMenuIcon('plugins')" class="admin-layout__menu-icon" /><span>{{ t('plugins.panels.overview') }}</span></span>
                      <span v-else-if="entry.kind === 'management'" class="admin-layout__menu-label"><component :is="resolveMenuIcon('plugin-settings')" class="admin-layout__menu-icon" /><span :title="entry.page.label">{{ entry.page.label }}</span></span>
                      <span v-else class="admin-layout__menu-label"><RotateCwIcon aria-hidden="true" /><span>{{ t('plugins.navigation.detailUnavailable') }} · {{ t('plugins.navigation.retry') }}</span></span>
                    </button>
                  </div>
                </div>
              </Transition>
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
  border: 1px solid transparent;
  border-radius: 999px;
  background: var(--sider-menu-active-bg);
  color: var(--sider-menu-active);
  box-shadow: var(--shadow-xs);
  cursor: pointer;
  font-size: 14px;
  font-weight: 700;
  text-align: left;
  transition: color var(--motion-fast) var(--motion-easing);
}

.sidebar-navigation__back:hover {
  color: var(--brand-foreground);
}

.sidebar-navigation__back:focus-visible {
  outline: 2px solid var(--focus);
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
  border-radius: 999px;
  background: transparent;
  color: var(--chrome-muted);
  cursor: pointer;
  font-size: 11px;
}

.sidebar-navigation__disclosure:hover {
  color: var(--sider-menu-text);
}

.sidebar-navigation__disclosure:focus-visible {
  outline: 2px solid var(--focus);
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
  color: var(--sider-menu-text);
  font-weight: 700;
}

// The open plugin is one block: its row and its pages share a neutral fill, so the white pill of the current page
// reads as part of that plugin. Other plugins keep their rows quiet and only show the fill on hover.
.sidebar-plugin {
  margin: 2px 4px;
  border-radius: 22px;
  transition: background-color var(--motion-content) var(--motion-easing), box-shadow var(--motion-content) var(--motion-easing);
}

// Like a segmented control, the open plugin is a recessed groove and its current page the raised pill inside it; the
// groove is darker than the sidebar in both themes, so it never blends with a hovered row or the selected pill.
.sidebar-plugin[data-active] {
  background: var(--surface-soft);
}

.sidebar-plugin > .sidebar-navigation__entry--resource {
  margin: 0;
}

// Pages hang from a thin guide under the plugin icon; their pills start where the plugin name starts.
.sidebar-plugin__pages {
  position: relative;
  display: grid;
  gap: 2px;
  padding: 2px 6px 6px 30px;
}

.sidebar-plugin__pages::before {
  position: absolute;
  top: 2px;
  bottom: 22px;
  left: 22px;
  width: 1px;
  background: color-mix(in srgb, var(--chrome-muted) 32%, transparent);
  content: '';
}

.sidebar-plugin__pages > .sidebar-navigation__item {
  width: 100%;
  min-height: 36px;
  margin: 0;
  padding: 7px 12px;
  font-size: var(--font-size-sm);
}

.sidebar-plugin__pages .admin-layout__menu-label {
  gap: 9px;
}

.sidebar-plugin__pages .admin-layout__menu-icon {
  width: 15px;
  height: 15px;
}

.sidebar-navigation__disclosure-icon {
  transition: rotate var(--motion-content) var(--motion-easing);
}

.sidebar-navigation__disclosure-icon[data-open] {
  rotate: 90deg;
}

// A list opens by growing its row track from zero, so everything below moves with it, and its pages arrive one after
// another; closing is quicker and takes the pages along without a stagger. The clip only exists while it moves, so
// the current page's shadow is never cut at rest.
.sidebar-collapse {
  display: grid;
  grid-template-rows: 1fr;
}

.sidebar-collapse__inner {
  min-height: 0;
}

.sidebar-collapse-enter-active,
.sidebar-collapse-leave-active {
  overflow: hidden;
}

.sidebar-collapse-enter-active {
  transition: grid-template-rows 260ms var(--motion-easing), opacity 200ms var(--motion-easing);
}

.sidebar-collapse-leave-active {
  transition: grid-template-rows 200ms cubic-bezier(0.4, 0, 0.2, 1), opacity 120ms cubic-bezier(0.4, 0, 1, 1);
}

.sidebar-collapse-enter-from,
.sidebar-collapse-leave-to {
  grid-template-rows: 0fr;
  opacity: 0;
}

.sidebar-collapse-enter-active .sidebar-navigation__item {
  animation: sidebar-navigation-reveal 240ms var(--motion-easing) both;
  animation-delay: calc(min(var(--reveal-order, 0), 6) * 22ms);
}

.sidebar-navigation__loading-icon {
  animation: sidebar-navigation-spin 900ms linear infinite;
}

.sidebar-navigation__filter {
  width: calc(100% - 8px);
  margin: 8px 4px 0;
  border-radius: 999px;
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

@media (pointer: coarse) {
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

  .sidebar-collapse-enter-active,
  .sidebar-collapse-leave-active,
  .sidebar-plugin,
  .sidebar-navigation__disclosure-icon {
    transition: none;
  }

  .sidebar-collapse-enter-active .sidebar-navigation__item {
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
// The group chevron matches the plugin center's chevron in size and right inset, so both sit in one column.
.sidebar-navigation__group-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; width: 100%; min-height: 36px; padding-inline-end: 16px; cursor: pointer; }
.sidebar-navigation__group-heading svg { transition: rotate var(--motion-content) var(--motion-easing); }
.sidebar-navigation__group-heading .is-collapsed { rotate: -90deg; }
// Items are quiet text rows; the current page is a white pill lifted by a soft shadow, with its icon in blue.
.sidebar-navigation__item { display: flex; align-items: center; width: calc(100% - 8px); min-height: 40px; margin: 2px 4px; padding: 8px 12px; border: 1px solid transparent; border-radius: 999px; color: var(--sider-menu-text); font-size: 14px; font-weight: 500; line-height: 1.4; text-align: left; cursor: pointer; transition: background-color var(--motion-fast) var(--motion-easing); }
.sidebar-navigation__item .admin-layout__menu-icon { color: var(--chrome-muted); }
.sidebar-navigation__item:hover { background: var(--sider-menu-hover-bg); color: var(--sider-menu-text); }
.sidebar-navigation__item:is([aria-current=page], [aria-current=true]) { background: var(--sider-menu-active-bg); color: var(--sider-menu-active); font-weight: 700; box-shadow: var(--shadow-xs); }
.sidebar-navigation__item:is([aria-current=page], [aria-current=true]) .admin-layout__menu-icon { color: var(--brand-foreground); }
.sidebar-navigation__item:focus-visible, .sidebar-navigation__group-heading:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.sidebar-navigation__entry { display: flex; align-items: center; min-width: 0; }
.sidebar-navigation__entry > .sidebar-navigation__item { flex: 1; min-width: 0; }
// A plugin row is one pill: the row takes the hover and open-plugin face, and its name and disclosure buttons stay transparent inside it.
.sidebar-navigation__entry--resource { margin: 2px 4px; border-radius: 999px; transition: background-color var(--motion-fast) var(--motion-easing); }
.sidebar-navigation__entry--resource:hover { background: var(--sider-menu-hover-bg); }
.sidebar-navigation__entry--resource > .sidebar-navigation__item, .sidebar-navigation__entry--resource > .sidebar-navigation__item:hover { width: auto; margin: 0; padding-right: 4px; background: transparent; }
@media (prefers-reduced-motion: reduce) { .sidebar-navigation__item, .sidebar-navigation__entry--resource, .sidebar-navigation__back, .sidebar-navigation__group-heading svg { transition: none; } }
@media (forced-colors: active) { .sidebar-navigation__item:is([aria-current=page], [aria-current=true]), .sidebar-navigation__back { border-color: Highlight; box-shadow: none; } }
.sidebar-navigation__item .admin-layout__menu-label > span { overflow: hidden; text-overflow: ellipsis; }
.sidebar-navigation__disclosure > svg, .sidebar-navigation__chevron { width: 16px; height: 16px; }
.sidebar-navigation__back > svg { width: 18px; height: 18px; }
.sidebar-navigation__item.sidebar-navigation__collapsed-item { width: 40px; height: 40px; justify-content: center; margin: 4px 0; padding: 10px; }
.sidebar-navigation[data-collapsed=true] .admin-layout__menu-label { justify-content: center; }
.sidebar-navigation__filter :deep(.app-input) { border-radius: 999px; background: var(--surface-raised); color: var(--sider-menu-text); border-color: transparent; box-shadow: var(--shadow-xs); font-size: 12px; }
@media (pointer: coarse) { .sidebar-navigation__item, .sidebar-navigation__group-heading { min-height: 44px; } }

</style>
