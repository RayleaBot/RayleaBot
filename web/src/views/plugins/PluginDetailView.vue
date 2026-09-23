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
import { ArrowLeftIcon } from '@lucide/vue'
import { computed, ref } from 'vue'
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
import { getErrorCodeMessage } from '@/lib/error-text'
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
  { value: 'summary', label: t('plugins.sections.runtimeSummary') },
  { value: 'commands', label: t('plugins.sections.commands') },
  { value: 'console', label: t('plugins.sections.console') },
])
const consoleFrameCount = computed(() => pluginConsoleStore.getConsole(pluginId.value).length)
const commandPrefix = computed(() => getPrimaryCommandPrefix(configDocument.value?.command?.prefixes))
const pluginDisplayName = computed(() => (
  currentPlugin.value?.name?.trim() || pluginsStore.getPluginDisplayName(pluginId.value)
))
const pluginPageTitle = computed(() => t('plugins.detailPageTitle', { name: pluginDisplayName.value }))

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
          class="plugin-header-segmented plugin-detail-panel-switch"
          :class="{ 'plugin-detail-panel-switch--sidebar-owned': !siderCollapsed }"
          @update:model-value="setActivePanelKey(String($event))"
        />
      </div>
    </template>

    <template #extra>
      <div class="table-actions plugin-detail-actions">
        <PluginPowerButton
          :checked="currentPlugin?.state !== 'disabled'"
          :loading="actionPending[pluginId] === 'enable' || actionPending[pluginId] === 'disable'"
          :disabled="!currentPlugin"
          :checked-label="t('plugins.actions.enable')"
          :unchecked-label="t('plugins.actions.disable')"
          @click="runAction(getToggleAction())"
        />
        <AppButton :disabled="!currentPlugin" :loading="actionPending[pluginId] === 'reload'" @click="runAction('reload')">{{ t('plugins.actions.reload') }}</AppButton>
        <AppButton :disabled="!currentPlugin" :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallDialogVisible = true" variant="destructive">{{ t('plugins.actions.uninstall') }}</AppButton>
      </div>
    </template>

    <AppAlert
      v-if="currentPlugin?.state_diagnosis?.kind === 'initialization_failed'"
      tone="danger"
      data-testid="plugin-initialization-failure"
      :title="t('plugins.initializationFailed')"
      :description="t('plugins.initializationFailureDescription', { reason: getErrorCodeMessage(currentPlugin.state_diagnosis.last_error_code) })"
    />

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
      <AppSkeleton v-if="detailLoading && !currentPlugin" :rows="4" />
      <PluginDetailHero v-else :plugin="currentPlugin" :plugin-id="pluginId" :plugin-name="pluginDisplayName" />

      <AppAlert
        v-if="currentPlugin?.trust?.level === 'unverified'"
        class="plugin-trust-attention"
        tone="warning"
        :title="t('plugins.trustAttention.title')"
        :description="t('plugins.trustAttention.description')"
      />

      <div class="plugin-detail-workspace">
        <main class="plugin-detail-main-column">
          <AppCard
            borderless
            class="plugin-detail-tab-card"
            :class="{ 'is-console-tab-active': activeDetailTab === 'console' }"
          >
            <AppTabs v-model="activeDetailTab" :items="detailTabs" keep-alive class="premium-detail-tabs">
              <template #tab="{ item }">
                <span class="premium-tab-label">
                  {{ item.label }}
                  <AppTag v-if="item.value === 'commands'" class="tab-badge">{{ currentPlugin?.commands?.length ?? 0 }}</AppTag>
                  <AppTag v-else-if="item.value === 'console'" class="tab-badge">{{ consoleFrameCount }}</AppTag>
                </span>
              </template>

              <template #summary>
                <PluginDetailSummary class="tab-pane-content" :plugin="currentPlugin" />
              </template>

              <template #commands>
                <div class="tab-pane-content plugin-console-tab-content">
                  <PluginCommandsPanel
                    :commands="currentPlugin?.commands ?? []"
                    :command-conflicts="currentPlugin?.command_conflicts ?? []"
                    :command-prefix="commandPrefix"
                  />
                </div>
              </template>

              <template #console>
                <PluginConsolePane
                  class="tab-pane-content"
                  :plugin-id="pluginId"
                  :active="activeDetailTab === 'console'"
                  :ready="readyToRenderHeavyContent"
                />
              </template>
            </AppTabs>
          </AppCard>
        </main>
      </div>
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

  <AppConfirmDialog :open="uninstallDialogVisible" :title="t('plugins.uninstallConfirmTitle')" :description="t('plugins.uninstallConfirmBody')" :busy="actionPending[pluginId] === 'uninstall'" danger :confirm-text="t('plugins.actions.uninstallConfirm')" :cancel-text="t('dashboard.previewCancel')" @confirm="uninstallPlugin" @cancel="uninstallDialogVisible = false" />
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
:deep(.app-card) {
  box-shadow: var(--shadow-xs);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface);
}

/* Tab Panel Styling */
.plugin-detail-tab-card {
  :deep(.app-card__body) {
    padding: 0;
  }

  &.is-console-tab-active {
    display: flex;
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden;

    :deep(.app-card__body),
    :deep(.app-tabs),
    :deep(.app-tabs__content),
    :deep(.app-tabs__content[data-state=active]) {
      display: flex;
      flex: 1 1 auto;
      flex-direction: column;
      min-height: 0;
      width: 100%;
    }

    .tab-pane-content {
      display: flex;
      flex: 1 1 auto;
      flex-direction: column;
      min-height: 0;
    }
  }
}

.plugin-detail-panel-switch {
  flex: 0 0 auto;
}

@media (min-width: #{bp.$navigation + 1px}) {
  .plugin-detail-panel-switch--sidebar-owned {
    display: none;
  }
}

.premium-detail-tabs {
  :deep(.app-tabs__header) {
    padding-inline: 18px;
    margin-bottom: 0;
    border-bottom: 1px solid var(--border);
  }

  :deep(.app-tabs__trigger) {
    padding-block: 14px;
  }
}

.premium-tab-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 0.92rem;
}

.tab-badge {
  font-family: var(--font-mono);
  font-size: 12px;
  padding-inline: 6px;
  border-radius: 4px;
}

.tab-pane-content {
  padding: 18px;
}

.plugin-console-tab-content {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* Actions in title */
.plugin-detail-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.plugin-detail-actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
}

/* Detail header switcher */
.plugin-page-title-layout {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;

  h1 {
    margin: 0 !important;
    font-size: clamp(1.12rem, 1.38vw, 1.32rem) !important;
    line-height: 1.25 !important;
    letter-spacing: -0.02em !important;
    font-weight: 700 !important;
  }
}

.plugin-detail-back-button {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 32px;
  min-width: 32px;
  height: 32px;
  padding: 0;
  border-radius: var(--radius-sm);
  color: var(--muted);

  &:hover,
  &:focus-visible {
    background: var(--surface-soft);
    color: var(--text);
  }
}

.plugin-header-segmented {
  font-size: 0.82rem;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  padding: 2px;

  :deep(.app-segmented__item) {
    border-radius: 4px;
    font-weight: 550;
    transition: border-color 120ms ease-out, background-color 120ms ease-out, color 120ms ease-out;
  }

  :deep(.app-segmented__item[data-state=checked]) {
    background: var(--surface);
    color: var(--text-accent);
    box-shadow: none;
  }
}

/* Workspace Structure */
.plugin-detail-overview {
  display: grid;
  gap: var(--app-page-toolbar-gap);
  min-height: 0;

  &.is-output-active {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    overflow: hidden;

    .plugin-detail-hero,
    .plugin-trust-attention {
      flex: 0 0 auto;
    }

    .plugin-detail-workspace,
    .plugin-detail-main-column {
      display: flex;
      flex: 1 1 auto;
      flex-direction: column;
      min-height: 0;
    }

    .plugin-detail-workspace {
      align-items: stretch;
    }
  }
}

.plugin-detail-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: start;
  gap: 16px;
}

.plugin-detail-main-column {
  display: grid;
  gap: 14px;
}

@media (max-width: #{bp.$compactPanel}) {
  .plugin-detail-back-button {
    width: 44px;
    min-width: 44px;
    height: 44px;
  }
}
</style>
