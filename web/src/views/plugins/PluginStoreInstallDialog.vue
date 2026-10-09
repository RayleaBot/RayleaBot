<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppAlert from '@/components/AppAlert.vue'
import AppButton from '@/components/AppButton.vue'
import AppCheckbox from '@/components/AppCheckbox.vue'
import AppDetailItem from '@/components/AppDetailItem.vue'
import AppDetails from '@/components/AppDetails.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppTag from '@/components/AppTag.vue'
import { t } from '@/i18n'
import { formatPluginVersion } from '@/lib/display'
import type { PluginStoreDependency, PluginStoreEntry } from '@/types/api'
import { pendingDependencies } from './plugin-store-dependencies'

// One dialog covers both prompts before a store install: the prerequisite plugins it needs or suggests, and the
// trusted-code confirmation for every plugin that is about to be installed for the first time.
const props = defineProps<{ open: boolean; plugin: PluginStoreEntry | null }>()
const emit = defineEmits<{ close: []; afterClose: []; confirm: [dependencyIds: string[]] }>()

// Recommended dependencies start ticked; the operator unticks the ones they do not want.
const skipped = ref(new Set<string>())
watch(() => props.plugin?.id, () => { skipped.value = new Set() })

const pending = computed(() => props.plugin ? pendingDependencies(props.plugin) : [])
const hasRequired = computed(() => pending.value.some(dependency => dependency.requirement === 'required'))
const blocking = computed(() => pending.value.filter(dependency => dependency.requirement === 'required' && dependency.state === 'unavailable'))
const selected = computed(() => pending.value.filter(dependency => dependency.state === 'installable'
  && (dependency.requirement === 'required' || !skipped.value.has(dependency.id))))
const confirmsTarget = computed(() => (props.plugin?.confirmation_reasons.length ?? 0) > 0)
const trustedNames = computed(() => [
  ...selected.value.map(dependency => dependency.name),
  ...(props.plugin && confirmsTarget.value ? [props.plugin.name] : []),
])
const isUpdate = computed(() => props.plugin?.install_state === 'update_available')
const title = computed(() => props.plugin
  ? t(isUpdate.value ? 'plugins.store.install.updateTitle' : 'plugins.store.install.title', { name: props.plugin.name })
  : '')
const confirmLabel = computed(() => {
  if (selected.value.length > 0 && blocking.value.length === 0) return t('plugins.store.install.confirmCount', { count: selected.value.length + 1 })
  if (trustedNames.value.length > 0) return t('plugins.store.confirm.action')
  return t(isUpdate.value ? 'plugins.store.actions.update' : 'plugins.store.actions.install')
})

function isSelected(dependency: PluginStoreDependency) {
  return dependency.requirement === 'required' || !skipped.value.has(dependency.id)
}

function toggle(dependency: PluginStoreDependency, value: boolean) {
  const next = new Set(skipped.value)
  if (value) next.delete(dependency.id)
  else next.add(dependency.id)
  skipped.value = next
}

function confirm() {
  if (blocking.value.length > 0) return
  emit('confirm', selected.value.map(dependency => dependency.id))
}
</script>

<template>
  <AppDialog :open="open" :title="title" fallback-focus="[data-testid=plugin-store-refresh]" @close="emit('close')" @after-close="emit('afterClose')">
    <template v-if="plugin">
      <section v-if="pending.length > 0" class="dependencies" :aria-label="t('plugins.store.install.dependenciesTitle')">
        <p class="dependencies__lead">
          {{ t(hasRequired ? 'plugins.store.install.requiredLead' : 'plugins.store.install.recommendedLead', { name: plugin.name }) }}
        </p>
        <ul class="dependency-list">
          <li v-for="dependency in pending" :key="dependency.id" class="dependency" :data-state="dependency.state">
            <div class="dependency__title">
              <AppCheckbox
                v-if="dependency.state === 'installable'"
                :model-value="isSelected(dependency)"
                :disabled="dependency.requirement === 'required'"
                :data-testid="`plugin-store-dependency-${dependency.id}`"
                @update:model-value="toggle(dependency, $event)"
              >
                <strong>{{ dependency.name }}</strong>
              </AppCheckbox>
              <strong v-else class="dependency__name">{{ dependency.name }}</strong>
              <AppTag :tone="dependency.requirement === 'required' ? 'info' : 'neutral'" size="small">
                {{ t(`plugins.store.install.${dependency.requirement}`) }}
              </AppTag>
            </div>
            <p v-if="dependency.reason" class="dependency__note">{{ dependency.reason }}</p>
            <p v-if="dependency.state === 'unavailable'" class="dependency__note dependency__note--unavailable" :data-required="dependency.requirement === 'required' || undefined">
              {{ t(dependency.requirement === 'required' ? 'plugins.store.install.unavailableRequired' : 'plugins.store.install.unavailableRecommended') }}
            </p>
          </li>
        </ul>
        <p v-if="selected.length > 0 && blocking.length === 0" class="dependencies__order">
          {{ t('plugins.store.install.order', { steps: [...selected.map(dependency => dependency.name), plugin.name].join(' → ') }) }}
        </p>
      </section>

      <AppAlert
        v-if="blocking.length > 0"
        tone="danger"
        :title="t('plugins.store.install.blockedTitle')"
        :description="t('plugins.store.install.blockedDescription', { names: blocking.map(dependency => dependency.name).join('、'), name: plugin.name })"
      />
      <template v-else-if="trustedNames.length > 0">
        <AppAlert
          tone="attention"
          :title="t('plugins.store.confirm.warning')"
          :description="t('plugins.store.confirm.description', { name: trustedNames.join('、') })"
        />
        <AppDetails v-if="confirmsTarget" class="confirm-details">
          <AppDetailItem :label="t('plugins.fields.version')">
            {{ formatPluginVersion(plugin.latest_release?.version) }}
          </AppDetailItem>
          <AppDetailItem :label="t('plugins.store.confirm.reason')">
            {{ plugin.confirmation_reasons.map(reason => t(`plugins.store.confirm.reasons.${reason}`)).join('、') }}
          </AppDetailItem>
        </AppDetails>
      </template>
    </template>
    <template #footer>
      <div class="flex justify-end gap-3">
        <AppButton @click="emit('close')">{{ t('shell.cancel') }}</AppButton>
        <AppButton variant="default" :disabled="blocking.length > 0" data-testid="plugin-store-install-confirm" @click="confirm">{{ confirmLabel }}</AppButton>
      </div>
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.dependencies { margin-bottom: 18px; }
.dependencies__lead { margin: 0 0 12px; color: var(--muted); line-height: 1.6; }

.dependency-list {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.dependency {
  padding: 12px 16px;
  background: var(--control-fill);
  border-radius: var(--app-card-radius);
}

.dependency__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dependency__title :deep(.app-checkbox) { min-height: 28px; }
.dependency__name { line-height: 28px; }

// Notes line up with the name, past the checkbox.
.dependency[data-state=installable] .dependency__note { padding-inline-start: 28px; }

.dependency__note {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}

.dependency__note--unavailable[data-required] { color: var(--text-danger); }

.dependencies__order {
  margin: 12px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.confirm-details { margin-top: 18px; }
</style>
