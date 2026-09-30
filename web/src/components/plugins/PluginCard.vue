<script setup lang="ts">
import { EyeIcon, RefreshCwIcon, SettingsIcon } from '@lucide/vue'
import { computed } from 'vue'

import AppButton from '@/components/AppButton.vue'
import AppStatusTag from '@/components/AppStatusTag.vue'
import AppTag from '@/components/AppTag.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import PluginPowerButton from '@/components/plugins/PluginPowerButton.vue'
import { t } from '@/i18n'
import { formatPluginVersion, getPluginStateLabel, getPluginTrustLabel } from '@/lib/display'
import { resolveStatusTone, type StatusTone } from '@/lib/status-tone'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginSummary } from '@/types/api'

// pendingAction: the lifecycle action currently running for this plugin, if any.
const props = defineProps<{ plugin: PluginSummary; pendingAction?: string | null }>()
defineEmits<{ detail: []; summary: []; manage: []; reload: []; toggle: [] }>()

const pluginsStore = usePluginsStore()
const description = computed(() => props.plugin.description?.trim() || t('display.empty'))
const sourceTypeLabel = computed(() => {
  switch (props.plugin.source?.package_source_type) {
    case 'local_zip': return t('plugins.localZip')
    case 'local_directory': return t('plugins.localDirectory')
    case 'remote_url': return t('plugins.remoteUrl')
    case 'catalog': return t('plugins.catalogSource')
    case 'development': return t('plugins.developmentSource')
    // Plugins without a recorded package source show only their trust level.
    default: return props.plugin.source?.package_source_type || ''
  }
})
const lifecycleSwitching = computed(() => props.plugin.state === 'starting' || props.plugin.state === 'stopping')
// Every plugin is a light gray box; stopped plugins dim their icon and problems get a ring in their tone.
const attentionTone = computed(() => {
  const tone = resolveStatusTone(props.plugin.state)
  return tone === 'danger' || tone === 'warning' ? tone : undefined
})
const toggleLoading = computed(() => props.pendingAction === 'enable' || props.pendingAction === 'disable' || lifecycleSwitching.value)
const reloadDisabled = computed(() => props.plugin.state === 'disabled' || lifecycleSwitching.value || props.plugin.state === 'invalid')

// At most three notices keep cards of equal height readable.
const healthNotices = computed(() => {
  const notices: Array<{ label: string; tone: StatusTone }> = []
  const conflicts = props.plugin.command_conflicts?.length ?? 0
  if (conflicts > 0) {
    notices.push({ label: t('plugins.health.commandConflicts', { count: conflicts }), tone: 'warning' })
  }
  if (props.plugin.source?.verified === false && props.plugin.trust?.level !== 'unverified') {
    notices.push({ label: t('plugins.health.unverifiedSource'), tone: 'neutral' })
  }
  if (props.plugin.state === 'failed') {
    notices.push({ label: t('plugins.health.runtimeIssue'), tone: 'danger' })
  } else if (props.plugin.state === 'invalid') {
    notices.push({ label: t('plugins.health.invalidManifest'), tone: 'danger' })
  } else if (props.plugin.state === 'enabled') {
    notices.push({ label: t('plugins.health.enabledButStopped'), tone: 'warning' })
  }
  return notices.slice(0, 3)
})
</script>

<template>
  <article class="plugin-grid-card app-box" :data-state="plugin.state" :data-attention="attentionTone">
    <header class="plugin-card__header">
      <PluginIcon :refresh-key="pluginsStore.iconRevision" :plugin-id="plugin.id" :icon="plugin.icon" :version="plugin.version" />
      <div class="plugin-card__identity">
        <button type="button" class="plugin-card__name" :title="plugin.name" @click="$emit('detail')">
          {{ plugin.name }}
        </button>
        <span v-if="plugin.version" class="plugin-card__version" :title="plugin.version">{{ formatPluginVersion(plugin.version) }}</span>
      </div>
      <AppStatusTag :status="plugin.state" :label="getPluginStateLabel(plugin.state)" :aria-label="t('plugins.stateAria', { state: getPluginStateLabel(plugin.state) })" />
    </header>

    <p class="plugin-card__description" :title="description">
      {{ description }}
    </p>

    <div class="plugin-card__meta">
      <span v-if="sourceTypeLabel">{{ sourceTypeLabel }}</span>
      <AppTag :tone="plugin.trust?.level === 'unverified' ? 'warning' : 'neutral'">{{ getPluginTrustLabel(plugin.trust?.level) }}</AppTag>
    </div>

    <div v-if="healthNotices.length > 0" class="plugin-health-notices">
      <AppTag
        v-for="notice in healthNotices"
        :key="notice.label"
        :tone="notice.tone"
        :aria-label="t('plugins.health.aria', { label: notice.label })"
      >
        {{ notice.label }}
      </AppTag>
    </div>

    <footer class="plugin-card__actions">
      <div class="plugin-card__action-buttons">
        <AppTooltip :title="t('plugins.actions.summary')">
          <AppButton class="plugin-card__icon-action" variant="ghost" :aria-label="t('plugins.actions.summary')" @click="$emit('summary')">
            <template #icon><EyeIcon /></template>
          </AppButton>
        </AppTooltip>
        <AppButton
          class="plugin-card__manage-action"
          variant="ghost"
          :aria-label="t('plugins.actions.manage')"
          :data-testid="`plugin-manage-button-${plugin.id}`"
          @click="$emit('manage')"
        >
          <template #icon><SettingsIcon /></template>
          {{ t('plugins.actions.manage') }}
        </AppButton>
        <AppTooltip :title="t('plugins.actions.reload')">
          <AppButton
            class="plugin-card__icon-action"
            variant="ghost"
            :aria-label="t('plugins.actions.reload')"
            :data-testid="`plugin-reload-button-${plugin.id}`"
            :loading="pendingAction === 'reload'"
            :disabled="reloadDisabled"
            @click="$emit('reload')"
          >
            <template #icon><RefreshCwIcon /></template>
          </AppButton>
        </AppTooltip>
      </div>
      <AppTooltip :title="plugin.state === 'disabled' ? t('plugins.actions.enable') : t('plugins.actions.disable')">
        <PluginPowerButton
          icon-only
          :checked="plugin.state !== 'disabled'"
          :data-testid="`plugin-enable-button-${plugin.id}`"
          :loading="toggleLoading"
          :checked-label="t('plugins.power.enabled')"
          :unchecked-label="t('plugins.power.disabled')"
          @click="$emit('toggle')"
        />
      </AppTooltip>
    </footer>
  </article>
</template>

<style lang="scss" scoped>
@use '@/styles/breakpoints.generated' as bp;

.plugin-grid-card {
  display: flex;
  min-width: 0;
  overflow: hidden;
  flex-direction: column;
  min-height: 224px;
  color: var(--text);
  transition: translate 160ms var(--motion-easing);
}

.plugin-grid-card[data-attention=warning] { box-shadow: inset 0 0 0 2px var(--warning), var(--shadow-card); }
.plugin-grid-card[data-attention=danger] { box-shadow: inset 0 0 0 2px var(--danger), var(--shadow-card); }
.plugin-grid-card[data-state=disabled] :deep(.plugin-icon) { filter: grayscale(1); opacity: .7; }

.plugin-grid-card:hover {
  translate: 0 -1px;
}

.plugin-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 16px 16px 12px;
}

.plugin-card__identity {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  min-width: 0;
  flex: 1 1 auto;
  gap: 2px 8px;
}

.plugin-card__name {
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text);
  cursor: pointer;
  font: inherit;
  font-size: 15px;
  font-weight: 700;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-card__name:hover,
.plugin-card__name:focus-visible {
  color: var(--brand-foreground);
}

.plugin-card__name:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: var(--focus-outline-offset);
  border-radius: 4px;
}

.plugin-card__version {
  max-width: 100%;
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-card__description {
  display: -webkit-box;
  min-height: 44px;
  margin: 0;
  overflow: hidden;
  padding: 0 16px;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.plugin-card__meta,
.plugin-health-notices {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 8px;
}

.plugin-card__meta {
  margin: 12px 16px 12px;
  min-height: 24px;
  padding: 0;
  border-radius: 0;
  background: transparent;
  color: var(--muted);
  font-size: 13px;
}

.plugin-card__meta > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-card__meta :deep(.app-tag),
.plugin-health-notices :deep(.app-tag) {
  margin-inline-end: 0;
}

.plugin-health-notices {
  padding: 0 16px 12px;
}

.plugin-card__actions,
.plugin-card__action-buttons {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.plugin-card__actions {
  justify-content: space-between;
  margin-top: auto;
  padding: 10px 12px;
  border-top: 1px solid var(--border);
  background: transparent;
}

.plugin-card__actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
}

.plugin-card__manage-action.app-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  padding-inline: 14px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: var(--control-fill);
  color: var(--text);
  font-size: 14px;
  box-shadow: var(--shadow-xs);
}
.plugin-card__manage-action.app-button:hover:not(:disabled) { background: var(--control-fill-hover); color: var(--text); }
.plugin-card__manage-action.app-button:active:not(:disabled) { transform: scale(.97); }
.plugin-card__icon-action.app-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  color: var(--text);
  background: var(--control-fill);
  border: 1px solid transparent;
  font-size: 18px;
  border-radius: 999px;
  box-shadow: var(--shadow-xs);
}
.plugin-card__icon-action.app-button:hover:not(:disabled) { background: var(--control-fill-hover); color: var(--text); }
.plugin-card__icon-action.app-button:active:not(:disabled) { transform: scale(.94); }
.plugin-card__icon-action.app-button:disabled { color: var(--muted); opacity: .45; box-shadow: none; }
@media (min-width: #{bp.$fiveColumns}) {
  .plugin-grid-card { min-height: 240px; }
}
@media (prefers-reduced-motion: reduce) {
  .plugin-grid-card { transition: none; }
  .plugin-grid-card:hover { translate: none; }
}
@media (forced-colors: active) {
  .plugin-grid-card[data-attention] { outline: 2px solid Highlight; outline-offset: -4px; }
}
@media (pointer: coarse) {
  .plugin-card__name { min-height: 44px; white-space: normal; line-height: 1.4; }
  .plugin-card__manage-action.app-button { min-height: 44px; }
  .plugin-card__icon-action.app-button { width: 44px; height: 44px; }
  .plugin-card__description { min-height: 0; }
}
</style>
