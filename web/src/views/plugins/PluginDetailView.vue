<script setup lang="ts">
import AppTabs from '@/components/AppTabs.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import AppTag from '@/components/AppTag.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppCard from '@/components/AppCard.vue'
import AppButton from '@/components/AppButton.vue'
import AppAlert from '@/components/AppAlert.vue'
import { ArrowLeftIcon, BlocksIcon, ScrollTextIcon, TerminalIcon } from '@lucide/vue'
import { computed, ref, type Component } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute } from 'vue-router'

import AppPage from '@/components/page/AppPage.vue'
import PluginConsolePane from '@/components/plugins/PluginConsolePane.vue'
import PluginDetailHero from '@/components/plugins/PluginDetailHero.vue'
import PluginDetailSummary from '@/components/plugins/PluginDetailSummary.vue'
import PluginManagementUIHost from '@/components/plugins/PluginManagementUIHost.vue'
import PluginPowerButton from '@/components/plugins/PluginPowerButton.vue'
import PluginCommandsPanel from '@/components/plugins/PluginCommandsPanel.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getPrimaryCommandPrefix } from '@/lib/command-usage'
import { canTogglePluginPower, getPluginReloadBlocker } from '@/lib/plugin-lifecycle'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { usePluginConsoleStore } from '@/stores/plugin-console'
import { usePluginsStore } from '@/stores/plugins'
import { useUiShellStore } from '@/stores/ui-shell'
import { useReadyToRenderHeavyContent } from '@/layouts/usePageTransitionStage'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import { usePluginDetail } from './usePluginDetail'
import { usePluginDetailPanels } from './usePluginDetailPanels'

type DetailTab = 'summary' | 'commands' | 'console'

const route = useRoute()
const navigate = useMotionNavigation()
const pluginsStore = usePluginsStore()
const pluginConsoleStore = usePluginConsoleStore()
const { document: configDocument } = storeToRefs(useConfigStore())
const { siderCollapsed } = storeToRefs(useUiShellStore())

const pluginId = computed(() => String(route.params.id))
const {
  actionPending,
  currentPlugin,
  detailLoading,
  getToggleAction,
  loadDetail,
  loadError,
  runAction,
  uninstallDialogVisible,
  uninstallPlugin,
} = usePluginDetail(pluginId)
const {
  activeManagementPage,
  activePanel,
  activePanelKey,
  managementPanelTitle,
  panelOptions,
  setActivePanelKey,
} = usePluginDetailPanels(pluginId, currentPlugin)
const readyToRenderHeavyContent = useReadyToRenderHeavyContent()

const activeDetailTab = ref<DetailTab>('summary')
const detailTabs = computed<{ value: DetailTab; label: string }[]>(() => [
  { value: 'summary', label: t('plugins.tabs.overview') },
  { value: 'commands', label: t('plugins.tabs.commands') },
  { value: 'console', label: t('plugins.tabs.console') },
])
// The overview tab uses the sidebar's overview icon, the commands tab the command center's.
const detailTabIcons: Record<DetailTab, Component> = { summary: BlocksIcon, commands: TerminalIcon, console: ScrollTextIcon }
const consoleFrameCount = computed(() => pluginConsoleStore.getConsole(pluginId.value).length)
const commandPrefix = computed(() => getPrimaryCommandPrefix(configDocument.value?.command?.prefixes))
const pluginDisplayName = computed(() => (
  currentPlugin.value?.name?.trim() || pluginsStore.getPluginDisplayName(pluginId.value)
))
const pluginPageTitle = computed(() => t('plugins.detailPageTitle', { name: pluginDisplayName.value }))
const reloadBlocker = computed(() => currentPlugin.value ? getPluginReloadBlocker(currentPlugin.value.state) : null)
const powerBlocked = computed(() => Boolean(currentPlugin.value && !canTogglePluginPower(currentPlugin.value.state)))

function returnToPluginList() {
  void navigate({ name: 'plugins' })
}
</script>

<template>
  <AppPage
    width="detail"
    :title="pluginPageTitle"
    :full-height="activePanel === 'management-ui' || (activePanel === 'overview' && activeDetailTab === 'console')"
  >
    <template #title>
      <div class="plugin-page-title-layout">
        <AppTooltip :title="t('plugins.actions.backToList')">
          <AppButton
            variant="ghost"
            size="icon"
            class="plugin-detail-back-button"
            :aria-label="t('plugins.actions.backToList')"
            data-testid="plugin-detail-back-button"
            @click="returnToPluginList"
          >
            <template #icon><ArrowLeftIcon /></template>
          </AppButton>
        </AppTooltip>
        <h1>{{ pluginPageTitle }}</h1>
        <AppSegmented
          v-if="panelOptions.length > 1"
          :model-value="activePanelKey"
          :options="panelOptions"
          class="plugin-detail-panel-switch"
          :class="{ 'plugin-detail-panel-switch--sidebar-owned': !siderCollapsed }"
          @update:model-value="setActivePanelKey(String($event))"
        />
      </div>
    </template>

    <template #extra>
      <div class="table-actions plugin-detail-actions">
        <AppTooltip v-if="powerBlocked" :title="t('plugins.power.invalid')">
          <span class="plugin-detail-actions__anchor">
            <PluginPowerButton :checked="true" disabled :checked-label="t('plugins.power.enabled')" :unchecked-label="t('plugins.power.disabled')" />
          </span>
        </AppTooltip>
        <PluginPowerButton
          v-else
          :checked="currentPlugin?.state !== 'disabled'"
          :loading="actionPending[pluginId] === 'enable' || actionPending[pluginId] === 'disable'"
          :disabled="!currentPlugin"
          :checked-label="t('plugins.power.enabled')"
          :unchecked-label="t('plugins.power.disabled')"
          @click="runAction(getToggleAction())"
        />
        <AppTooltip v-if="reloadBlocker" :title="reloadBlocker">
          <span class="plugin-detail-actions__anchor">
            <AppButton disabled>{{ t('plugins.actions.reload') }}</AppButton>
          </span>
        </AppTooltip>
        <AppButton v-else :disabled="!currentPlugin" :loading="actionPending[pluginId] === 'reload'" @click="runAction('reload')">{{ t('plugins.actions.reload') }}</AppButton>
        <AppButton :disabled="!currentPlugin" :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallDialogVisible = true" variant="destructive">{{ t('plugins.actions.uninstall') }}</AppButton>
      </div>
    </template>

    <RetryPanel
      v-if="loadError && !currentPlugin"
      :title="t('errors.common.loadFailed')"
      :description="loadError"
      :loading="detailLoading"
      @retry="loadDetail()"
    />

    <div
      v-if="activePanel === 'overview'"
      class="plugin-detail-overview"
      :class="{ 'is-output-active': activeDetailTab === 'console' }"
    >
      <AppSkeleton v-if="detailLoading && !currentPlugin" :rows="2" />
      <PluginDetailHero v-else :plugin="currentPlugin" :plugin-id="pluginId" :plugin-name="pluginDisplayName" />

      <AppAlert
        v-if="currentPlugin?.trust?.level === 'unverified'"
        class="plugin-trust-attention"
        tone="attention"
        :title="t('plugins.trustAttention.title')"
        :description="t('plugins.trustAttention.description')"
      />

      <!-- The tabs sit on the white page; the overview brings its own boxes, the other tabs one box each. -->
      <AppTabs v-model="activeDetailTab" :items="detailTabs" :label="t('plugins.tabs.label')" keep-alive class="plugin-detail-tabs">
        <template #tab="{ item }">
          <span class="plugin-detail-tab">
            <component :is="detailTabIcons[item.value]" class="plugin-detail-tab__icon" aria-hidden="true" />
            {{ item.label }}
            <AppTag v-if="item.value === 'commands'" class="tab-badge">{{ currentPlugin?.commands?.length ?? 0 }}</AppTag>
            <AppTag v-else-if="item.value === 'console'" class="tab-badge">{{ consoleFrameCount }}</AppTag>
          </span>
        </template>

        <template #summary>
          <PluginDetailSummary
            v-if="currentPlugin"
            :plugin="currentPlugin"
            :plugin-id="pluginId"
            :reload-pending="actionPending[pluginId] === 'reload'"
            @reload="runAction('reload')"
            @open-tab="activeDetailTab = $event"
          />
          <AppSkeleton v-else-if="detailLoading" :rows="6" />
        </template>

        <template #commands>
          <div class="app-box plugin-detail-tab-box">
            <PluginCommandsPanel
              :commands="currentPlugin?.commands ?? []"
              :command-conflicts="currentPlugin?.command_conflicts ?? []"
              :command-prefix="commandPrefix"
            />
          </div>
        </template>

        <template #console>
          <div class="app-box plugin-detail-tab-box plugin-detail-console-box">
            <PluginConsolePane
              class="plugin-detail-console"
              :plugin-id="pluginId"
              :plugin-state="currentPlugin?.state"
              :active="activeDetailTab === 'console'"
              :ready="readyToRenderHeavyContent"
            />
          </div>
        </template>
      </AppTabs>
    </div>

    <PluginManagementUIHost
      v-else-if="currentPlugin?.management_ui && activeManagementPage"
      :plugin="currentPlugin"
      :title="managementPanelTitle"
      :page="activeManagementPage"
    />

    <AppCard v-else borderless :loading="detailLoading">
      <template #title>
        <div class="card-header">
          <span>{{ managementPanelTitle }}</span>
        </div>
      </template>
    </AppCard>
  </AppPage>

  <AppConfirmDialog :open="uninstallDialogVisible" :title="t('plugins.uninstallConfirmTitle')" :description="t('plugins.uninstallConfirmBody', { name: pluginDisplayName })" :busy="actionPending[pluginId] === 'uninstall'" danger :confirm-text="t('plugins.actions.uninstallConfirm')" :cancel-text="t('dashboard.previewCancel')" @confirm="uninstallPlugin" @cancel="uninstallDialogVisible = false" />
</template>

<style scoped lang="scss">
.plugin-detail-panel-switch {
  flex: 0 0 auto;
}

.plugin-detail-panel-switch--sidebar-owned {
  display: none;
}

// Page-level tabs carry an icon, the label and a count, and sit closer to their content than the default header.
.plugin-detail-tabs :deep(.app-tabs__header) {
  margin-bottom: 16px;
}

.plugin-detail-tab {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.plugin-detail-tab__icon {
  width: 16px;
  height: 16px;
}

.tab-badge {
  font-family: var(--font-mono);
  font-size: 12px;
}

// The tab counts sit on the page too; on the dark page they take the raised fill like the identity tags.
:global([data-theme='dark']) .plugin-detail-tab .tab-badge {
  background: var(--surface-raised);
}

.plugin-detail-tab-box {
  min-width: 0;
  padding: 18px 20px;
}

/* Actions in title */
.plugin-detail-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

// A disabled button takes no pointer events, so its tooltip hangs on a wrapper.
.plugin-detail-actions__anchor {
  display: inline-flex;
}

.plugin-detail-actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
  --button-height: 40px;
}

/* Detail header switcher */
.plugin-page-title-layout {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.plugin-detail-back-button {
  flex: 0 0 auto;
  color: var(--muted);
}

.plugin-detail-overview {
  display: grid;
  gap: var(--app-page-toolbar-gap);
  min-height: 0;

  // The live output fills the rest of the page, so every layer down to the console pane flexes.
  &.is-output-active {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    overflow: hidden;

    .plugin-detail-hero,
    .plugin-trust-attention {
      flex: 0 0 auto;
    }

    .plugin-detail-tabs,
    .plugin-detail-tabs :deep(.app-tabs__content[data-state=active]),
    .plugin-detail-console-box,
    .plugin-detail-console {
      display: flex;
      flex: 1 1 auto;
      flex-direction: column;
      min-height: 0;
    }
  }
}
</style>
