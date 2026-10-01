<script setup lang="ts">
import { computed } from 'vue'

import AppTag from '@/components/AppTag.vue'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import { t } from '@/i18n'
import { formatPluginVersion, getPluginRoleLabel, getPluginTrustLabel } from '@/lib/display'
import { buildPluginWorkbenchActions } from '@/lib/management-links'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail } from '@/types/api'

const props = defineProps<{ plugin: PluginDetail | null; pluginId: string; pluginName: string }>()

const pluginsStore = usePluginsStore()
const workbenchActions = computed(() => buildPluginWorkbenchActions(props.pluginId))
const requiresTrustAttention = computed(() => props.plugin?.trust?.level === 'unverified')
const author = computed(() => props.plugin?.author?.trim())
</script>

<template>
  <!-- The plugin's identity sits on the white page above the tabs: mark, name and tags, then ID, version and author. -->
  <section class="plugin-detail-hero" :aria-label="t('plugins.overview.identity')">
    <span class="plugin-detail-hero__mark">
      <PluginIcon
        :refresh-key="pluginsStore.iconRevision"
        data-testid="plugin-detail-icon"
        :plugin-id="pluginId"
        :icon="plugin?.icon"
        :version="plugin?.version"
      />
    </span>
    <div class="plugin-detail-hero__copy">
      <p class="plugin-detail-hero__title">
        <strong class="plugin-title">{{ pluginName }}</strong>
        <AppTag size="small">{{ getPluginRoleLabel(plugin?.role) }}</AppTag>
        <AppTag size="small" :tone="requiresTrustAttention ? 'attention' : 'neutral'">{{ getPluginTrustLabel(plugin?.trust?.level) }}</AppTag>
      </p>
      <p class="plugin-detail-hero__meta">
        <span class="plugin-id-sub">{{ pluginId }}</span>
        <template v-if="plugin?.version?.trim()"><span aria-hidden="true">·</span><span>{{ formatPluginVersion(plugin.version) }}</span></template>
        <template v-if="author"><span aria-hidden="true">·</span><span>{{ t('plugins.overview.author', { author }) }}</span></template>
      </p>
    </div>
    <ManagementContextActions :actions="workbenchActions" />
  </section>
</template>

<style scoped lang="scss">
.plugin-detail-hero {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  padding: 4px 2px 0;
}

// The mark keeps a light gray tile of its own, lifted like the controls on the page.
.plugin-detail-hero__mark {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: var(--surface);
  box-shadow: var(--shadow-xs);
}

.plugin-detail-hero__copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.plugin-detail-hero__title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  margin: 0;
}

.plugin-title {
  overflow: hidden;
  color: var(--text);
  font-size: var(--font-size-xl);
  font-weight: 700;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-detail-hero__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0 6px;
  min-width: 0;
  margin: 0;
  color: var(--muted);
  font-size: var(--font-size-sm);
}

.plugin-id-sub {
  font-family: var(--font-mono);
  overflow-wrap: anywhere;
}

// The soft tag fill nearly matches the dark page, so neutral tags on the page take the raised fill there.
:global([data-theme='dark']) .plugin-detail-hero__title .app-tag[data-tone=neutral] {
  background: var(--surface-raised);
}

@media (forced-colors: active) {
  .plugin-detail-hero__mark { border: 1px solid CanvasText; box-shadow: none; }
}
</style>
