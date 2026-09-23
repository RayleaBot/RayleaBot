<script setup lang="ts">
import { computed } from 'vue'

import AppTag from '@/components/AppTag.vue'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import { t } from '@/i18n'
import { formatPluginVersion, getPluginRoleLabel, getPluginStateLabel, getPluginTrustLabel } from '@/lib/display'
import { buildPluginWorkbenchActions } from '@/lib/management-links'
import type { StatusTone } from '@/lib/status-tone'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail } from '@/types/api'

const props = defineProps<{ plugin: PluginDetail | null; pluginId: string; pluginName: string }>()

const pluginsStore = usePluginsStore()
const workbenchActions = computed(() => buildPluginWorkbenchActions(props.pluginId))
const requiresTrustAttention = computed(() => props.plugin?.trust?.level === 'unverified')
const stateTone = computed(() => getPluginStateTone(props.plugin?.state))
const facts = computed(() => [
  { key: 'version', label: t('plugins.fields.version'), value: formatPluginVersion(props.plugin?.version) },
  { key: 'core', label: t('plugins.fields.minCoreVersion'), value: formatPluginVersion(props.plugin?.min_core_version) },
  { key: 'source', label: t('plugins.fields.sourceRoot'), value: props.plugin?.source?.root?.trim() || t('display.empty') },
])

function getPluginStateTone(status?: string | null): StatusTone {
  if (!status) return 'neutral'
  if (status === 'failed' || status === 'error' || status === 'removed') return 'danger'
  if (status === 'starting' || status === 'stopping' || status === 'enabling' || status === 'disabling' || status === 'retrying') return 'warning'
  if (status === 'installed' || status === 'enabled' || status === 'running' || status === 'discovered') return 'success'
  return 'neutral'
}
</script>

<template>
  <section class="plugin-detail-hero">
    <div class="plugin-detail-hero__identity">
      <PluginIcon
        :refresh-key="pluginsStore.iconRevision"
        class="plugin-detail-hero__avatar"
        data-testid="plugin-detail-icon"
        :plugin-id="pluginId"
        :icon="plugin?.icon"
        :version="plugin?.version"
      />
      <div class="plugin-detail-hero__copy">
        <div class="plugin-detail-hero__badges">
          <AppTag class="premium-badge role-badge">{{ getPluginRoleLabel(plugin?.role) }}</AppTag>
          <AppTag class="premium-badge trust-badge" :class="{ 'is-attention': requiresTrustAttention }">{{ getPluginTrustLabel(plugin?.trust?.level) }}</AppTag>
        </div>
        <strong class="plugin-title">{{ pluginName }}</strong>
        <span class="plugin-id-sub">{{ pluginId }}</span>
      </div>
    </div>

    <div class="plugin-detail-hero__tools">
      <ManagementContextActions :actions="workbenchActions" />
    </div>

    <div class="plugin-detail-status-chips" :aria-label="t('plugins.sections.statusSummary')">
      <div class="status-chip">
        <span class="status-chip__dot" :style="{ backgroundColor: stateTone === 'neutral' ? 'var(--muted)' : `var(--${stateTone})` }"></span>
        <span class="status-chip__label">{{ t('plugins.fields.state') }}:</span>
        <AppTag :tone="stateTone" class="status-tag">
          {{ getPluginStateLabel(plugin?.state) }}
          <small v-if="plugin?.state"> · {{ plugin.state }}</small>
        </AppTag>
      </div>
    </div>

    <dl class="plugin-detail-hero__facts">
      <div v-for="item in facts" :key="item.key" class="fact-item">
        <dt class="fact-label">{{ item.label }}</dt>
        <dd class="fact-value">{{ item.value }}</dd>
      </div>
    </dl>
  </section>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;

.plugin-detail-hero {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 12px 20px;
  padding: 14px 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: none;
}

.plugin-detail-hero__identity {
  grid-area: 1 / 1 / 2 / 2;
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 12px;
}

.plugin-detail-hero__avatar.plugin-icon {
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
}

.plugin-detail-hero__avatar :deep(.raylea-mark) {
  width: 34px;
  height: 34px;
}

.plugin-detail-hero__copy {
  display: grid;
  grid-template-areas:
    "title badges"
    "id id";
  grid-template-columns: auto 1fr;
  align-items: center;
  min-width: 0;
  gap: 2px 8px;
}

.plugin-title {
  grid-area: title;
  overflow: hidden;
  color: var(--text);
  font-size: 1.15rem;
  font-weight: 600;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-id-sub {
  grid-area: id;
  overflow: hidden;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-detail-hero__badges {
  grid-area: badges;
  display: flex;
  align-items: center;
  gap: 4px;
}

.premium-badge {
  font-size: 12px;
  font-weight: 600;
  border-radius: 4px;
  padding-inline: 6px;
  margin-inline-end: 0 !important;
}

.role-badge {
  background: color-mix(in srgb, var(--accent) 8%, transparent);
  color: var(--accent);
  border: 1px solid color-mix(in srgb, var(--accent) 15%, transparent);
}

.trust-badge {
  background: color-mix(in srgb, var(--success) 8%, transparent);
  color: var(--success);
  border: 1px solid color-mix(in srgb, var(--success) 12%, transparent);
}

.trust-badge.is-attention {
  color: var(--text-attention);
  background: var(--surface-attention);
  border-color: var(--border-attention);
}

.plugin-detail-hero__tools {
  grid-area: 1 / 2 / 2 / 3;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.plugin-detail-status-chips {
  grid-area: 2 / 1 / 3 / 2;
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  padding: 6px 10px;
  border: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface-soft) 40%, transparent);
  align-items: center;
}

.status-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.status-chip__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
  display: inline-block;
}

.status-chip__label {
  color: var(--muted);
  font-weight: 550;
}

/* Compact status tag inside the detail header */
.status-tag.app-tag {
  font-family: var(--font-mono);
  font-size: 12px;
  padding-inline: 6px;
  margin-inline-end: 0 !important;
  border-radius: 4px;
  font-weight: 600;
  border: 1px solid transparent;
  line-height: 1.5;
  background: transparent;
  color: inherit;

  small {
    font-size: 12px;
    opacity: 0.8;
  }
}

.plugin-detail-hero__facts {
  grid-area: 2 / 2 / 3 / 3;
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  margin: 0;
  align-items: center;
  justify-content: flex-end;
}

.fact-item {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  background: color-mix(in srgb, var(--surface-soft) 20%, transparent);
  padding: 3px 6px;
  border-radius: var(--radius-sm);
  border: 1px dashed color-mix(in srgb, var(--border) 40%, transparent);
}

.fact-label {
  color: var(--muted);
  font-weight: 550;
  text-transform: none;
  letter-spacing: 0;

  &::after {
    content: ':';
  }
}

.fact-value {
  min-width: 0;
  margin: 0;
  color: var(--text);
  font-size: 12px;
  font-weight: 600;
  font-family: var(--font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: #{bp.$pluginSettings}) {
  .plugin-detail-hero {
    grid-template-columns: 1fr;
  }

  .plugin-detail-hero__tools {
    justify-content: flex-start;
  }
}
</style>
