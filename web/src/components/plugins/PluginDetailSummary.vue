<script setup lang="ts">
import { computed } from 'vue'
import {
  ActivityIcon,
  ArrowRightIcon,
  ArrowUpRightIcon,
  BookOpenIcon,
  CheckIcon,
  ChevronRightIcon,
  CircleDashedIcon,
  FolderInputIcon,
  GitBranchIcon,
  GlobeIcon,
  HashIcon,
  HistoryIcon,
  ImageIcon,
  ImagesIcon,
  LayersIcon,
  MinusIcon,
  PackageIcon,
  RadioIcon,
  RotateCwIcon,
  ScaleIcon,
  SettingsIcon,
  ShieldIcon,
  SlidersHorizontalIcon,
  TagIcon,
  TerminalIcon,
  UserIcon,
  WebhookIcon,
} from '@lucide/vue'

import AppButton from '@/components/AppButton.vue'
import AppTag from '@/components/AppTag.vue'
import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import { describePluginHealth } from '@/components/plugins/plugin-health'
import { t } from '@/i18n'
import { apiPath } from '@/lib/api-path'
import { formatPluginVersion, getEventTypeLabel, getPluginSourceTypeLabel, getPluginTrustLabel } from '@/lib/display'
import { buildLogsLocation, buildPluginDetailLocation } from '@/lib/management-links'
import { getPluginCommandAvailability } from '@/lib/plugin-commands'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import type { PluginDetail } from '@/types/api'

const props = defineProps<{ plugin: PluginDetail; pluginId: string; reloadPending?: boolean }>()
const emit = defineEmits<{ reload: []; openTab: [tab: 'commands' | 'console'] }>()

const navigate = useMotionNavigation()
const health = computed(() => describePluginHealth(props.plugin))
// Warnings and errors are what the health box points at; the identity row already opens the full history.
const warningLogs = computed(() => buildLogsLocation({ history: true, filters: { pluginIds: [props.pluginId], levels: ['warn', 'error'] } }))

const events = computed(() => (props.plugin.events ?? []).map(name => ({ name, label: getEventTypeLabel(name) })))
const eventSummary = computed(() => events.value.map(event => event.label ?? event.name).join('、'))

const commandCount = computed(() => props.plugin.commands?.length ?? 0)
const conflictCount = computed(() => props.plugin.command_conflicts?.length ?? 0)
const webhookPaths = computed(() => (props.plugin.webhooks ?? []).map(webhook => ({
  id: webhook.id,
  path: apiPath('/api/webhooks/{plugin_id}/{route}', { plugin_id: props.pluginId, route: webhook.route }),
})))
const managementPages = computed(() => props.plugin.management_ui?.pages ?? [])

const trustLevel = computed(() => props.plugin.trust?.level)
// Every package sits under the same install root, so the row only says how this one was installed and the path
// or address its package came from.
const installMethod = computed(() => ({
  label: getPluginSourceTypeLabel(props.plugin.source?.package_source_type),
  reference: props.plugin.source?.package_source_ref?.trim(),
}))
const handling = computed(() => {
  // Omitted values take the manifest defaults: one event at a time, priority 0, and propagation continues.
  const concurrency = props.plugin.concurrency ?? 1
  return {
    value: t('plugins.overview.origin.handlingValue', {
      concurrency,
      priority: props.plugin.priority ?? 0,
      propagation: t(`plugins.overview.origin.propagation.${props.plugin.block ? 'stop' : 'continue'}`),
    }),
    note: t('plugins.overview.origin.handlingNote', { concurrency }),
  }
})

// Project facts the manifest declares get a row each; the ones it leaves out share the last row.
const projectFields = computed(() => {
  const plugin = props.plugin
  return [
    { key: 'author', label: t('plugins.fields.author'), present: Boolean(plugin.author?.trim()) },
    { key: 'license', label: t('plugins.fields.license'), present: Boolean(plugin.license?.trim()) },
    { key: 'repo', label: t('plugins.fields.repo'), present: Boolean(plugin.repo) },
    { key: 'homepage', label: t('plugins.fields.homepage'), present: Boolean(plugin.homepage) },
    { key: 'keywords', label: t('plugins.fields.keywords'), present: hasItems(plugin.keywords) },
    { key: 'screenshots', label: t('plugins.fields.screenshots'), present: hasItems(plugin.screenshots) },
    { key: 'icon', label: t('plugins.fields.icon'), present: Boolean(plugin.icon?.trim()) },
  ]
})
const declared = computed(() => new Set(projectFields.value.filter(field => field.present).map(field => field.key)))
const undeclaredLabels = computed(() => projectFields.value.filter(field => !field.present).map(field => field.label))

function hasItems(value?: readonly unknown[] | null) {
  return Array.isArray(value) && value.length > 0
}

// Links read as their address without the scheme; the target keeps the full URL.
function displayUrl(url?: string) {
  return (url ?? '').replace(/^https?:\/\//, '').replace(/\/$/, '')
}
</script>

<template>
  <!-- Four boxes, each answering one question. Left: what the plugin is doing. Right: where it comes from. -->
  <div class="plugin-board">
    <div class="plugin-board__column">
      <section class="app-box plugin-board__box" aria-labelledby="plugin-health-title" data-testid="plugin-health" :data-problem="health.problem || undefined">
        <h2 id="plugin-health-title" class="plugin-board__title"><ActivityIcon aria-hidden="true" />{{ t('plugins.overview.health.title') }}</h2>
        <div class="plugin-health-state">
          <span class="plugin-health-state__tile" :data-tone="health.tone"><component :is="health.icon" aria-hidden="true" /></span>
          <div class="plugin-health-state__copy">
            <p class="plugin-health-state__title" :data-tone="health.tone">{{ health.title }}</p>
            <p class="plugin-health-state__description">{{ health.description }}</p>
          </div>
        </div>

        <template v-if="health.problem">
          <dl v-if="health.facts.length" class="plugin-kv plugin-health-facts">
            <template v-for="fact in health.facts" :key="fact.key">
              <dt><component :is="fact.icon" aria-hidden="true" />{{ fact.label }}</dt>
              <dd>
                <template v-if="fact.value">{{ fact.value }}</template>
                <span v-if="fact.code" class="plugin-kv__note is-mono">{{ fact.code }}</span>
                <span v-for="path in fact.paths" :key="path" class="plugin-kv__path is-mono">{{ path }}</span>
              </dd>
            </template>
          </dl>
          <div class="plugin-health-actions">
            <AppButton v-if="health.canReload" variant="default" :loading="reloadPending" @click="emit('reload')">
              <template #icon><RotateCwIcon /></template>
              {{ t('plugins.overview.health.reloadNow') }}
            </AppButton>
            <AppButton @click="navigate(warningLogs)">
              <template #icon><HistoryIcon /></template>
              {{ t('plugins.overview.health.relatedLogs') }}
            </AppButton>
          </div>
        </template>

        <template v-else>
          <ul v-if="plugin.state === 'running'" class="plugin-health-checks">
            <li><CheckIcon aria-hidden="true" />{{ t('plugins.overview.health.noDiagnosis') }}<span>{{ t('plugins.overview.health.noDiagnosisNote') }}</span></li>
            <li v-if="events.length"><CheckIcon aria-hidden="true" />{{ t('plugins.overview.health.subscriptions') }}<span>{{ eventSummary }}</span></li>
            <li v-else><MinusIcon class="is-neutral" aria-hidden="true" />{{ t('plugins.overview.health.noSubscriptions') }}</li>
          </ul>
          <div class="plugin-health-links">
            <button type="button" class="plugin-board__link" @click="emit('openTab', 'console')">{{ t('plugins.overview.health.liveOutput') }}<ChevronRightIcon aria-hidden="true" /></button>
            <MotionRouterLink :to="warningLogs" class="plugin-board__link">{{ t('plugins.overview.health.warningLogs') }}<ChevronRightIcon aria-hidden="true" /></MotionRouterLink>
          </div>
        </template>
      </section>

      <section class="app-box plugin-board__box" aria-labelledby="plugin-function-title" data-testid="plugin-function">
        <h2 id="plugin-function-title" class="plugin-board__title"><LayersIcon aria-hidden="true" />{{ t('plugins.overview.function.title') }}</h2>
        <p v-if="plugin.description?.trim()" class="plugin-function__lede">{{ plugin.description }}</p>
        <p v-else class="plugin-function__lede is-muted">{{ t('plugins.overview.function.noDescription') }}</p>

        <!-- Commands are a count that leads to the commands tab, never a list on the overview. -->
        <dl class="plugin-function__commands">
          <dt><TerminalIcon aria-hidden="true" />{{ t('plugins.overview.function.commands') }}</dt>
          <dd v-if="commandCount" class="plugin-function__command-summary">
            <span>{{ t('plugins.overview.function.commandCount', { count: commandCount }) }}</span>
            <span class="is-muted">{{ t(`commands.status.${getPluginCommandAvailability(plugin)}`) }}</span>
            <span v-if="conflictCount" class="is-warning">{{ t('plugins.health.commandConflicts', { count: conflictCount }) }}</span>
            <span v-else class="is-muted">{{ t('plugins.overview.function.noConflicts') }}</span>
          </dd>
          <dd v-else class="is-muted">{{ t('plugins.overview.undeclared') }}</dd>
          <dd v-if="commandCount">
            <AppButton size="sm" @click="emit('openTab', 'commands')">
              {{ t('plugins.overview.function.viewCommands') }}
              <ArrowRightIcon class="plugin-function__arrow" aria-hidden="true" />
            </AppButton>
          </dd>
        </dl>

        <div class="plugin-function__facts">
          <dl class="plugin-fact">
            <dt><RadioIcon aria-hidden="true" />{{ t('plugins.overview.function.events') }}</dt>
            <dd v-if="events.length" class="plugin-board__tags">
              <AppTag v-for="event in events" :key="event.name">
                <template v-if="event.label">{{ event.label }}</template>
                <span class="is-mono">{{ event.name }}</span>
              </AppTag>
            </dd>
            <dd v-else class="is-muted">{{ t('plugins.overview.undeclared') }}</dd>
          </dl>
          <dl class="plugin-fact">
            <dt><WebhookIcon aria-hidden="true" />{{ t('plugins.fields.webhooks') }}</dt>
            <dd v-if="webhookPaths.length" class="plugin-fact__list">
              <span v-for="webhook in webhookPaths" :key="webhook.id" class="is-mono">{{ webhook.path }}</span>
            </dd>
            <dd v-else class="is-muted">{{ t('plugins.overview.undeclared') }}</dd>
          </dl>
          <dl class="plugin-fact">
            <dt><SettingsIcon aria-hidden="true" />{{ t('plugins.overview.function.managementPages') }}</dt>
            <dd v-if="managementPages.length" class="plugin-fact__list">
              <MotionRouterLink
                v-for="page in managementPages"
                :key="page.id"
                :to="buildPluginDetailLocation(pluginId, { panel: 'management-ui', managementPage: page.id })"
                class="plugin-board__link"
              >
                {{ page.label?.trim() || t('plugins.panels.managementUi') }}<ChevronRightIcon aria-hidden="true" />
              </MotionRouterLink>
            </dd>
            <dd v-else class="is-muted">{{ t('plugins.overview.undeclared') }}</dd>
          </dl>
        </div>
      </section>
    </div>

    <div class="plugin-board__column">
      <section class="app-box plugin-board__box" aria-labelledby="plugin-origin-title" data-testid="plugin-origin">
        <h2 id="plugin-origin-title" class="plugin-board__title"><PackageIcon aria-hidden="true" />{{ t('plugins.overview.origin.title') }}</h2>
        <dl class="plugin-kv">
          <dt><FolderInputIcon aria-hidden="true" />{{ t('plugins.fields.sourceRef') }}</dt>
          <dd>
            <template v-if="installMethod.label">{{ installMethod.label }}</template>
            <span v-else class="is-muted">{{ t('plugins.overview.origin.methodUnrecorded') }}</span>
            <span v-if="installMethod.reference" class="plugin-kv__note is-mono">{{ installMethod.reference }}</span>
          </dd>
          <dt><ShieldIcon aria-hidden="true" />{{ t('plugins.fields.trust') }}</dt>
          <dd>
            {{ getPluginTrustLabel(trustLevel) }}
            <span v-if="trustLevel !== 'unverified' && plugin.source" class="plugin-kv__note">{{ t(plugin.source.verified ? 'plugins.overview.origin.sourceVerified' : 'plugins.overview.origin.sourceUnverified') }}</span>
          </dd>
          <dt><TagIcon aria-hidden="true" />{{ t('plugins.fields.version') }}</dt>
          <dd>
            {{ formatPluginVersion(plugin.version) }}
            <span v-if="plugin.min_core_version?.trim()" class="plugin-kv__note">{{ t('plugins.overview.origin.requiresCore', { version: formatPluginVersion(plugin.min_core_version) }) }}</span>
          </dd>
          <dt><SlidersHorizontalIcon aria-hidden="true" />{{ t('plugins.overview.origin.handling') }}</dt>
          <dd>
            {{ handling.value }}
            <span class="plugin-kv__note">{{ handling.note }}</span>
          </dd>
        </dl>
      </section>

      <section class="app-box plugin-board__box" aria-labelledby="plugin-project-title" data-testid="plugin-project">
        <h2 id="plugin-project-title" class="plugin-board__title"><BookOpenIcon aria-hidden="true" />{{ t('plugins.overview.project.title') }}</h2>
        <dl class="plugin-kv">
          <template v-if="declared.has('author')">
            <dt><UserIcon aria-hidden="true" />{{ t('plugins.fields.author') }}</dt>
            <dd>{{ plugin.author }}</dd>
          </template>
          <template v-if="declared.has('license')">
            <dt><ScaleIcon aria-hidden="true" />{{ t('plugins.fields.license') }}</dt>
            <dd class="is-mono">{{ plugin.license }}</dd>
          </template>
          <template v-if="declared.has('repo')">
            <dt><GitBranchIcon aria-hidden="true" />{{ t('plugins.fields.repo') }}</dt>
            <dd><a :href="plugin.repo" target="_blank" rel="noreferrer" class="plugin-board__link">{{ displayUrl(plugin.repo) }}<ArrowUpRightIcon aria-hidden="true" /></a></dd>
          </template>
          <template v-if="declared.has('homepage')">
            <dt><GlobeIcon aria-hidden="true" />{{ t('plugins.fields.homepage') }}</dt>
            <dd><a :href="plugin.homepage" target="_blank" rel="noreferrer" class="plugin-board__link">{{ displayUrl(plugin.homepage) }}<ArrowUpRightIcon aria-hidden="true" /></a></dd>
          </template>
          <template v-if="declared.has('keywords')">
            <dt><HashIcon aria-hidden="true" />{{ t('plugins.fields.keywords') }}</dt>
            <dd class="plugin-board__tags"><AppTag v-for="keyword in plugin.keywords" :key="keyword">{{ keyword }}</AppTag></dd>
          </template>
          <template v-if="declared.has('screenshots')">
            <dt><ImagesIcon aria-hidden="true" />{{ t('plugins.fields.screenshots') }}</dt>
            <dd>
              <span v-for="screenshot in plugin.screenshots" :key="screenshot.path" class="plugin-kv__screenshot">
                <span class="is-mono">{{ screenshot.path }}</span>
                <span v-if="screenshot.alt?.trim()" class="plugin-kv__note">{{ screenshot.alt }}</span>
              </span>
            </dd>
          </template>
          <template v-if="declared.has('icon')">
            <dt><ImageIcon aria-hidden="true" />{{ t('plugins.fields.icon') }}</dt>
            <dd class="is-mono">{{ plugin.icon }}</dd>
          </template>
          <template v-if="undeclaredLabels.length">
            <dt><CircleDashedIcon aria-hidden="true" />{{ t('plugins.overview.undeclared') }}</dt>
            <dd class="is-muted">{{ undeclaredLabels.join('、') }}</dd>
          </template>
        </dl>
      </section>
    </div>
  </div>
</template>

<style scoped lang="scss">
// Two columns of different weight: what the plugin is doing on the left, where it comes from on the right.
.plugin-board {
  display: grid;
  grid-template-columns: minmax(0, 7fr) minmax(0, 5fr);
  align-items: start;
  gap: 16px;
}

.plugin-board__column {
  display: grid;
  gap: 16px;
  min-width: 0;
}

.plugin-board__box {
  min-width: 0;
  padding: 18px 22px 20px;
}

.plugin-board__title {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 28px;
  margin: 0 0 10px;
  color: var(--text);
  font-size: var(--font-size-lg);
  font-weight: 700;
  line-height: 1.35;

  svg {
    width: 18px;
    height: 18px;
    color: var(--muted);
  }
}

.plugin-board__link {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 0;
  border: 0;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--brand-foreground);
  font: inherit;
  font-size: var(--font-size-sm);
  font-weight: 500;
  cursor: pointer;

  &:hover {
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  &:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  svg {
    flex: none;
    width: 15px;
    height: 15px;
  }
}

.plugin-board__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

// Scoped under the board so they win over the value styles of the rows they sit in.
.plugin-board .is-mono {
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  font-weight: 400;
}

.plugin-board .is-muted {
  color: var(--muted);
  font-weight: 400;
}

.plugin-board .is-warning {
  color: var(--text-warning);
}

// Health: one state line with its tile, then either the routine checks or what the server recorded.
.plugin-health-state {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 4px 0 14px;
}

.plugin-health-state__tile {
  display: grid;
  flex: none;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: var(--surface-soft);
  color: var(--muted);

  svg {
    width: 20px;
    height: 20px;
    stroke-width: 2.1;
  }

  &[data-tone=success] { background: var(--surface-success); color: var(--text-success); }
  &[data-tone=info] { background: var(--surface-info); color: var(--text-info); }
  &[data-tone=warning] { background: var(--surface-warning); color: var(--text-warning); }
  &[data-tone=attention] { background: var(--surface-attention); color: var(--text-attention); }
  &[data-tone=danger] { background: var(--surface-danger); color: var(--text-danger); }
}

.plugin-health-state__copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.plugin-health-state__title {
  margin: 0;
  color: var(--text);
  font-size: var(--font-size-xl);
  font-weight: 700;
  line-height: 1.3;

  &[data-tone=danger] { color: var(--text-danger); }
}

.plugin-health-state__description {
  margin: 0;
  color: var(--muted);
  font-size: var(--font-size-md);
}

.plugin-health-checks {
  display: grid;
  margin: 0;
  padding: 0;
  list-style: none;

  li {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 44px;
    padding: 10px 0;
    border-top: 1px solid var(--border);
    font-size: var(--font-size-md);
  }

  svg {
    flex: none;
    width: 16px;
    height: 16px;
    color: var(--text-success);
    stroke-width: 2.4;
  }

  svg.is-neutral {
    color: var(--muted);
  }

  span {
    min-width: 0;
    margin-left: auto;
    color: var(--muted);
    font-size: var(--font-size-sm);
    text-align: right;
  }
}

.plugin-health-links {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  padding-top: 12px;
  border-top: 1px solid var(--border);
}

.plugin-health-facts {
  border-top: 1px solid var(--border);
}

.plugin-health-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}

// Key-value rows divided by unbroken lines; labels take their own width so values start on one edge.
.plugin-kv {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  margin: 0;

  dt,
  dd {
    min-height: 44px;
    padding: 11px 0;
    border-top: 1px solid var(--border);
  }

  dt:first-of-type,
  dd:first-of-type {
    border-top: 0;
  }

  dt {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding-right: 24px;
    color: var(--muted);
    font-size: var(--font-size-sm);
    line-height: 22px;

    svg {
      flex: none;
      width: 15px;
      height: 15px;
      margin-top: 3.5px;
    }
  }

  dd {
    min-width: 0;
    margin: 0;
    color: var(--text);
    font-size: var(--font-size-md);
    font-weight: 500;
    line-height: 22px;
    overflow-wrap: anywhere;
  }

  dd .plugin-board__link {
    font-size: inherit;
  }
}

.plugin-kv__note {
  display: block;
  color: var(--muted);
  font-size: var(--font-size-sm);
  font-weight: 400;
  line-height: 20px;
}

.plugin-kv__path,
.plugin-kv__screenshot {
  display: block;
}

.plugin-kv__screenshot + .plugin-kv__screenshot {
  margin-top: 6px;
}

// Function: the plugin's own description, the command count with its way in, then three declared facts.
.plugin-function__lede {
  max-width: 62ch;
  margin: 0;
  color: var(--text);
  font-size: var(--font-size-md);
  line-height: 1.6;
}

.plugin-function__commands {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  margin: 16px 0 0;
  padding: 12px 0;
  border-block: 1px solid var(--border);

  dt {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--muted);
    font-size: var(--font-size-sm);

    svg {
      width: 15px;
      height: 15px;
    }
  }

  dd {
    margin: 0;
    font-size: var(--font-size-md);
    font-weight: 500;
  }
}

.plugin-function__command-summary {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0 8px;

  span + span::before {
    content: '·';
    margin-right: 8px;
    color: var(--muted);
  }
}

.plugin-function__arrow {
  width: 15px;
  height: 15px;
}

.plugin-function__facts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  padding-top: 14px;
}

.plugin-fact {
  display: grid;
  align-content: start;
  gap: 6px;
  min-width: 0;
  margin: 0;

  dt {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--muted);
    font-size: var(--font-size-sm);

    svg {
      flex: none;
      width: 15px;
      height: 15px;
    }
  }

  dd {
    min-width: 0;
    margin: 0;
    font-size: var(--font-size-md);
    font-weight: 500;
    overflow-wrap: anywhere;
  }
}

.plugin-fact__list {
  display: grid;
  justify-items: start;
  gap: 4px;
}
</style>
