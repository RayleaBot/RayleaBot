<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import {
  CheckCircleOutlined,
  GithubOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import { usePluginStore, type PluginStoreSort } from '@/stores/plugin-store'
import type { PluginStoreEntry } from '@/types/api'

const store = usePluginStore()
const { catalog, error, installing, items, loading, refreshing, total } = storeToRefs(store)

const query = ref('')
const sort = ref<PluginStoreSort>('recommended')
const confirmationOpen = ref(false)
const selectedPlugin = ref<PluginStoreEntry | null>(null)

const catalogSourceLabel = computed(() => (
  catalog.value?.source === 'remote'
    ? t('plugins.store.catalog.remote')
    : t('plugins.store.catalog.embedded')
))

async function loadEntries() {
  try {
    await store.fetchEntries({ query: query.value, sort: sort.value })
  } catch {
    // The store owns the page error state.
  }
}

async function loadInitialEntries() {
  await loadEntries()
  if (catalog.value?.source !== 'embedded') return
  try {
    await store.refreshCatalog()
  } catch {
    // Keep the release-signed embedded catalog when remote refresh is unavailable.
  }
}

async function refreshCatalog() {
  try {
    await store.refreshCatalog()
    notifySuccess(t('plugins.store.feedback.refreshed'))
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
}

function requestInstall(plugin: PluginStoreEntry) {
  selectedPlugin.value = plugin
  confirmationOpen.value = true
}

async function confirmInstall() {
  const plugin = selectedPlugin.value
  if (!plugin) return

  try {
    await store.install(plugin.id, plugin.latest_release?.version)
    confirmationOpen.value = false
    selectedPlugin.value = null
    notifySuccess(t('plugins.store.feedback.accepted'))
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
}

function installActionLabel(plugin: PluginStoreEntry) {
  switch (plugin.install_state) {
    case 'update_available': return t('plugins.store.actions.update')
    case 'installed': return t('plugins.store.actions.installed')
    case 'unpublished': return t('plugins.store.actions.unpublished')
    case 'incompatible': return t('plugins.store.actions.incompatible')
    default: return t('plugins.store.actions.install')
  }
}

function canInstall(plugin: PluginStoreEntry) {
  return plugin.install_state === 'available' || plugin.install_state === 'update_available'
}

onMounted(() => {
  void loadInitialEntries()
})
</script>

<template>
  <AppPage
    :title="t('plugins.store.title')"
    :description="t('plugins.store.description')"
  >
    <template #status>
      <a-tag v-if="catalog" color="blue">
        <SafetyCertificateOutlined />
        {{ t('plugins.store.catalog.verified') }} · {{ catalogSourceLabel }}
      </a-tag>
    </template>

    <template #extra>
      <a-button :loading="refreshing" @click="refreshCatalog">
        <template #icon><ReloadOutlined /></template>
        {{ t('plugins.store.actions.refresh') }}
      </a-button>
    </template>

    <RetryPanel
      v-if="error && items.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadEntries"
    />

    <template v-else>
      <AppCard borderless class="store-toolbar-card">
        <div class="store-toolbar">
          <a-input-search
            v-model:value="query"
            :placeholder="t('plugins.store.searchPlaceholder')"
            allow-clear
            class="store-search"
            @search="loadEntries"
          >
            <template #prefix><SearchOutlined /></template>
          </a-input-search>
          <a-select v-model:value="sort" class="store-sort" @change="loadEntries">
            <a-select-option value="recommended">{{ t('plugins.store.sort.recommended') }}</a-select-option>
            <a-select-option value="name">{{ t('plugins.store.sort.name') }}</a-select-option>
            <a-select-option value="updated">{{ t('plugins.store.sort.updated') }}</a-select-option>
          </a-select>
          <span class="store-count">{{ t('plugins.store.resultCount', { count: total }) }}</span>
        </div>
      </AppCard>

      <a-skeleton v-if="loading" active :paragraph="{ rows: 8 }" />
      <AppEmptyState
        v-else-if="items.length === 0"
        :title="t('plugins.store.empty.title')"
        :description="t('plugins.store.empty.description')"
      />
      <div v-else class="store-grid">
        <AppCard
          v-for="plugin in items"
          :key="plugin.id"
          class="store-plugin-card"
          shadow="sm"
        >
          <div class="plugin-card-header">
            <div class="plugin-identity">
              <div class="plugin-mark">{{ plugin.name.slice(0, 2).toUpperCase() }}</div>
              <div>
                <div class="plugin-title-line">
                  <h2>{{ plugin.name }}</h2>
                  <a-tag v-if="plugin.recommended" color="processing">
                    {{ t('plugins.store.recommended') }}
                  </a-tag>
                </div>
                <span class="plugin-id">{{ plugin.id }}</span>
              </div>
            </div>
            <a-tooltip :title="t('plugins.store.repository')">
              <a-button
                :href="plugin.repository_url"
                target="_blank"
                rel="noopener noreferrer"
                shape="circle"
                type="text"
                :aria-label="t('plugins.store.repository')"
              >
                <template #icon><GithubOutlined /></template>
              </a-button>
            </a-tooltip>
          </div>

          <p class="plugin-summary">{{ plugin.summary }}</p>

          <div class="plugin-meta">
            <span>
              <CheckCircleOutlined v-if="plugin.publisher.verified" class="verified-icon" />
              {{ plugin.publisher.name }}
            </span>
            <span>{{ plugin.license }}</span>
            <span v-if="plugin.latest_release">v{{ plugin.latest_release.version }}</span>
          </div>

          <div class="plugin-card-footer">
            <span v-if="plugin.installed_version" class="installed-version">
              {{ t('plugins.store.installedVersion', { version: plugin.installed_version }) }}
            </span>
            <span v-else />
            <a-button
              type="primary"
              :disabled="!canInstall(plugin)"
              :loading="installing[plugin.id]"
              :data-testid="`plugin-store-install-${plugin.id}`"
              @click="requestInstall(plugin)"
            >
              {{ installActionLabel(plugin) }}
            </a-button>
          </div>
        </AppCard>
      </div>
    </template>

    <a-modal
      v-model:open="confirmationOpen"
      :title="t('plugins.store.confirm.title')"
      :confirm-loading="selectedPlugin ? installing[selectedPlugin.id] : false"
      :ok-text="t('plugins.store.confirm.action')"
      :cancel-text="t('shell.cancel')"
      @ok="confirmInstall"
    >
      <a-alert
        type="warning"
        show-icon
        :message="t('plugins.store.confirm.warning')"
        :description="t('plugins.store.confirm.description', { name: selectedPlugin?.name ?? '' })"
      />
      <dl v-if="selectedPlugin" class="confirm-details">
        <div><dt>{{ t('plugins.fields.id') }}</dt><dd>{{ selectedPlugin.id }}</dd></div>
        <div><dt>{{ t('plugins.fields.version') }}</dt><dd>{{ selectedPlugin.latest_release?.version }}</dd></div>
        <div><dt>{{ t('plugins.store.publisher') }}</dt><dd>{{ selectedPlugin.publisher.name }}</dd></div>
      </dl>
    </a-modal>
  </AppPage>
</template>

<style scoped lang="scss">
.store-toolbar-card {
  margin-bottom: 18px;
}

.store-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.store-search {
  width: min(440px, 100%);
}

.store-sort {
  width: 150px;
}

.store-count {
  margin-left: auto;
  color: var(--muted);
  font-size: 13px;
}

.store-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 340px), 1fr));
  gap: 16px;
}

.store-plugin-card :deep(.ant-card-body) {
  display: flex;
  min-height: 244px;
  flex-direction: column;
}

.plugin-card-header,
.plugin-identity,
.plugin-title-line,
.plugin-meta,
.plugin-card-footer {
  display: flex;
  align-items: center;
}

.plugin-card-header {
  justify-content: space-between;
  gap: 12px;
}

.plugin-identity {
  min-width: 0;
  gap: 12px;
}

.plugin-mark {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: var(--radius-lg);
  color: var(--primary);
  background: color-mix(in srgb, var(--primary) 12%, var(--surface));
  font-weight: 700;
}

.plugin-title-line {
  gap: 8px;
}

.plugin-title-line h2 {
  margin: 0;
  color: var(--text);
  font-size: 17px;
  line-height: 1.35;
}

.plugin-id,
.plugin-meta,
.installed-version {
  color: var(--muted);
  font-size: 12px;
}

.plugin-summary {
  margin: 18px 0 14px;
  color: var(--text-secondary, var(--muted));
  line-height: 1.65;
}

.plugin-meta {
  flex-wrap: wrap;
  gap: 8px 14px;
}

.verified-icon {
  color: var(--success);
}

.plugin-card-footer {
  justify-content: space-between;
  gap: 12px;
  margin-top: auto;
  padding-top: 20px;
}

.confirm-details {
  margin: 18px 0 0;
}

.confirm-details div {
  display: grid;
  grid-template-columns: 90px minmax(0, 1fr);
  gap: 12px;
  padding: 6px 0;
}

.confirm-details dt {
  color: var(--muted);
}

.confirm-details dd {
  margin: 0;
  overflow-wrap: anywhere;
}

@media (max-width: 640px) {
  .store-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .store-search,
  .store-sort {
    width: 100%;
  }

  .store-count {
    margin-left: 0;
  }
}
</style>
