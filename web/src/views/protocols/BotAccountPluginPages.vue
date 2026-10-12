<script setup lang="ts">
import { computed, ref } from 'vue'
import { LayoutGridIcon } from '@lucide/vue'

import AppButton from '@/components/AppButton.vue'
import AppDropdown from '@/components/AppDropdown.vue'
import AppDropdownItem from '@/components/AppDropdownItem.vue'
import { t } from '@/i18n'
import { buildPluginDetailLocation } from '@/lib/management-links'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import { usePluginsStore } from '@/stores/plugins'
import type { BotAccountRef } from '@/types/api'

const props = defineProps<{ account: BotAccountRef; disabled?: boolean }>()

const navigate = useMotionNavigation()
const pluginsStore = usePluginsStore()
const loading = ref(false)
const failed = ref(false)

// The plugin list does not declare pages, so details are read when the menu opens rather than for every visit.
async function loadPages() {
  loading.value = true
  failed.value = false
  try {
    await pluginsStore.ensureList()
    await Promise.all(pluginsStore.items.map(plugin => pluginsStore.ensureDetail(plugin.id)))
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}

const pages = computed(() => pluginsStore.sortedItems.flatMap((plugin) => {
  const pluginPages = pluginsStore.detailsByPluginId[plugin.id]?.management_ui?.pages ?? []
  return pluginPages
    .filter(page => page.scope === 'bot_account')
    .map(page => ({
      key: `${plugin.id}:${page.id}`,
      label: `${pluginsStore.getPluginDisplayName(plugin.id)} · ${page.label}`,
      to: buildPluginDetailLocation(plugin.id, { panel: 'management-ui', managementPage: page.id, bot: props.account }),
    }))
}))
</script>

<template>
  <AppDropdown :label="t('protocols.botAccountPages.label')" align="end">
    <AppButton size="sm" :disabled="disabled" data-testid="bot-account-plugin-pages" @click="loadPages">
      <template #icon><LayoutGridIcon /></template>{{ t('protocols.botAccountPages.label') }}
    </AppButton>
    <template #content>
      <AppDropdownItem v-if="loading && pages.length === 0" disabled>{{ t('protocols.botAccountPages.loading') }}</AppDropdownItem>
      <AppDropdownItem v-else-if="failed && pages.length === 0" disabled>{{ t('protocols.botAccountPages.failed') }}</AppDropdownItem>
      <AppDropdownItem v-else-if="pages.length === 0" disabled>{{ t('protocols.botAccountPages.empty') }}</AppDropdownItem>
      <AppDropdownItem v-for="page in pages" :key="page.key" data-testid="bot-account-plugin-page" @select="navigate(page.to)">{{ page.label }}</AppDropdownItem>
    </template>
  </AppDropdown>
</template>
