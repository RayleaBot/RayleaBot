<script setup lang="ts">
import { ArrowUpRightIcon } from '@lucide/vue'
import AppButton from '@/components/AppButton.vue'
import AppCard from '@/components/AppCard.vue'
import AppDetailItem from '@/components/AppDetailItem.vue'
import AppDetails from '@/components/AppDetails.vue'
import AppDrawer from '@/components/AppDrawer.vue'
import AppStatusTag from '@/components/AppStatusTag.vue'
import AppTag from '@/components/AppTag.vue'
import PluginCommandsPanel from '@/components/plugins/PluginCommandsPanel.vue'
import { t } from '@/i18n'
import { getPluginInstallMethodLabel, getPluginRoleLabel, getPluginStateLabel, getPluginTrustLabel } from '@/lib/display'
import { buildPluginDetailLocation } from '@/lib/management-links'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import type { PluginSummary } from '@/types/api'

defineProps<{ open: boolean; plugin: PluginSummary | null }>()
const emit = defineEmits<{ close: [] }>()
const navigate = useMotionNavigation()

function openDetail(pluginId: string) {
  emit('close')
  void navigate(buildPluginDetailLocation(pluginId))
}
</script>

<template>
  <!-- The drawer is named after its plugin; the facts follow as divided rows. -->
  <AppDrawer :open="open" :title="plugin?.name || t('plugins.actions.summary')" :width="560" @close="$emit('close')">
    <template v-if="plugin">
      <AppDetails>
        <AppDetailItem :label="t('plugins.fields.id')"><span class="plugin-summary-drawer__mono">{{ plugin.id }}</span></AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.state')"><AppStatusTag :label="getPluginStateLabel(plugin.state)" :status="plugin.state" /></AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.role')">{{ getPluginRoleLabel(plugin.role) }}</AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.trust')">{{ getPluginTrustLabel(plugin.trust?.level) }}</AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.sourceRoot')"><span class="plugin-summary-drawer__mono">{{ plugin.source?.root ?? t('display.empty') }}</span></AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.sourceRef')">{{ getPluginInstallMethodLabel(plugin.source) }}</AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.conflicts')">
          <div v-if="plugin.command_conflicts?.length" class="table-actions">
            <AppTag v-for="command in plugin.command_conflicts" :key="command" tone="warning">
              {{ command }}
            </AppTag>
          </div>
          <span v-else>{{ t('display.empty') }}</span>
        </AppDetailItem>
      </AppDetails>

      <AppCard :title="t('plugins.sections.commands')" variant="flat" class="drawer-card">
        <PluginCommandsPanel
          stacked
          :commands="plugin.commands"
          :command-conflicts="plugin.command_conflicts"
        />
      </AppCard>
    </template>
    <template v-if="plugin" #footer>
      <div class="plugin-summary-drawer__footer">
        <AppButton data-testid="plugin-summary-open-detail" @click="openDetail(plugin.id)">
          {{ t('plugins.actions.openDetail') }}
          <ArrowUpRightIcon class="plugin-summary-drawer__arrow" aria-hidden="true" />
        </AppButton>
      </div>
    </template>
  </AppDrawer>
</template>

<style lang="scss" scoped>
// The drawer is already a box, so its sections sit flat on it and only dividers separate them.
.drawer-card {
  margin-top: 12px;
}

.drawer-card :deep(.app-card__head) {
  padding: 12px 0;
}

.drawer-card :deep(.app-card__body) {
  padding: 12px 0 0;
}

.plugin-summary-drawer__mono {
  font-family: var(--font-mono);
}

.plugin-summary-drawer__footer {
  display: flex;
  justify-content: flex-end;
}

.plugin-summary-drawer__arrow {
  width: 14px;
  height: 14px;
  color: var(--muted);
}
</style>
