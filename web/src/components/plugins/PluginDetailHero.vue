<script setup lang="ts">
import { computed } from 'vue'

import AppTag from '@/components/AppTag.vue'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import { t } from '@/i18n'
import { formatPluginVersion, getPluginRoleLabel, getPluginStateLabel, getPluginTrustLabel } from '@/lib/display'
import { buildPluginWorkbenchActions } from '@/lib/management-links'
import { resolveStatusTone } from '@/lib/status-tone'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail } from '@/types/api'

const props = defineProps<{ plugin: PluginDetail | null; pluginId: string; pluginName: string }>()

const pluginsStore = usePluginsStore()
const workbenchActions = computed(() => buildPluginWorkbenchActions(props.pluginId))
const requiresTrustAttention = computed(() => props.plugin?.trust?.level === 'unverified')
const stateTone = computed(() => resolveStatusTone(props.plugin?.state))
const facts = computed(() => [
  { key: 'version', label: t('plugins.fields.version'), value: formatPluginVersion(props.plugin?.version) },
])
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
          <AppTag size="small">{{ getPluginRoleLabel(plugin?.role) }}</AppTag>
          <AppTag size="small" :tone="requiresTrustAttention ? 'attention' : 'neutral'">{{ getPluginTrustLabel(plugin?.trust?.level) }}</AppTag>
        </div>
        <strong class="plugin-title">{{ pluginName }}</strong>
        <span class="plugin-id-sub">{{ pluginId }}</span>
      </div>
    </div>

    <div class="plugin-detail-hero__tools">
      <ManagementContextActions :actions="workbenchActions" />
    </div>

    <p class="plugin-detail-status">
      <span class="plugin-detail-status__dot" :style="{ backgroundColor: stateTone === 'neutral' ? 'var(--muted)' : `var(--${stateTone})` }" aria-hidden="true"></span>
      <span class="plugin-detail-status__label">{{ t('plugins.fields.state') }}</span>
      <span class="plugin-detail-status__value">{{ getPluginStateLabel(plugin?.state) }}</span>
    </p>

    <dl class="plugin-detail-hero__facts">
      <div v-for="item in facts" :key="item.key" class="fact-item">
        <dt class="fact-label">{{ item.label }}</dt>
        <dd class="fact-value">{{ item.value }}</dd>
      </div>
    </dl>
  </section>
</template>

<style scoped lang="scss">
// One light gray box: identity and workbench links on the first row, state and package facts as plain text below.
.plugin-detail-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 14px 20px;
  padding: 18px 20px;
  border: 1px solid transparent;
  border-radius: var(--app-card-radius);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.plugin-detail-hero__identity {
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
  font-size: 18px;
  font-weight: 700;
  line-height: 1.3;
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
  gap: 6px;
}

.plugin-detail-hero__tools {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.plugin-detail-status {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  margin: 0;
  font-size: 13px;
}

.plugin-detail-status__dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.plugin-detail-status__label,
.fact-label {
  color: var(--muted);
}

.plugin-detail-status__value {
  color: var(--text);
  font-weight: 600;
}


.plugin-detail-hero__facts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 6px 20px;
  margin: 0;
}

.fact-item {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
  font-size: 13px;
}

.fact-value {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (forced-colors: active) {
  .plugin-detail-hero { border-color: CanvasText; box-shadow: none; }
  .plugin-detail-status__dot { background: CanvasText !important; }
}
</style>
