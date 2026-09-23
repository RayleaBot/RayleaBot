<script setup lang="ts">
import AppCard from '@/components/AppCard.vue'
import AppDetailItem from '@/components/AppDetailItem.vue'
import AppDetails from '@/components/AppDetails.vue'
import AppDrawer from '@/components/AppDrawer.vue'
import AppTag from '@/components/AppTag.vue'
import PluginCommandsPanel from '@/components/plugins/PluginCommandsPanel.vue'
import { t } from '@/i18n'
import { getPluginRoleLabel, getPluginStateLabel, getPluginTrustLabel } from '@/lib/display'
import type { PluginSummary } from '@/types/api'

defineProps<{ open: boolean; plugin: PluginSummary | null }>()
defineEmits<{ close: [] }>()
</script>

<template>
  <AppDrawer :open="open" :title="t('plugins.actions.summary')" :width="560" @close="$emit('close')">
    <template v-if="plugin">
      <div class="drawer-section">
        <div class="mono-list">
          <strong>{{ plugin.name }}</strong>
          <small>{{ plugin.id }}</small>
        </div>
      </div>

      <AppCard borderless class="drawer-card">
        <AppDetails>
          <AppDetailItem :label="t('plugins.fields.role')">{{ getPluginRoleLabel(plugin.role) }}</AppDetailItem>
          <AppDetailItem :label="t('plugins.fields.trust')">{{ getPluginTrustLabel(plugin.trust?.level) }}</AppDetailItem>
          <AppDetailItem :label="t('plugins.fields.state')">{{ getPluginStateLabel(plugin.state) }}</AppDetailItem>
          <AppDetailItem :label="t('plugins.fields.source')">{{ plugin.source?.root ?? t('display.empty') }}</AppDetailItem>
          <AppDetailItem :label="t('plugins.fields.sourceRef')">
            {{ plugin.source?.package_source_ref ?? plugin.source?.package_source_type ?? t('display.empty') }}
          </AppDetailItem>
          <AppDetailItem :label="t('plugins.fields.conflicts')">
            <div v-if="plugin.command_conflicts?.length" class="table-actions">
              <AppTag v-for="command in plugin.command_conflicts" :key="command" tone="warning">
                {{ command }}
              </AppTag>
            </div>
            <span v-else>{{ t('display.empty') }}</span>
          </AppDetailItem>
        </AppDetails>
      </AppCard>

      <AppCard :title="t('plugins.sections.commands')" borderless class="drawer-card">
        <PluginCommandsPanel
          :commands="plugin.commands"
          :command-conflicts="plugin.command_conflicts"
        />
      </AppCard>
    </template>
  </AppDrawer>
</template>

<style lang="scss" scoped>
.drawer-card {
  margin-top: 12px;
}

.drawer-section {
  margin-top: 24px;
  padding: var(--space-lg) 0;
  border-bottom: 1px solid var(--border);
}

.mono-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
  font-family: var(--font-mono);
  font-size: 13px;

  strong { font-size: 1rem; font-weight: 600; }
  small { font-family: var(--font-mono); font-size: 13px; color: var(--muted); }
}
</style>
