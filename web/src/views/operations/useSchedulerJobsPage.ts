import { onActivated, onDeactivated, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatNextRunRelative } from '@/lib/scheduler-job-display'
import { usePluginsStore } from '@/stores/plugins'
import { useSchedulerJobsStore } from '@/stores/scheduler-jobs'
import type { SchedulerJobSummary } from '@/types/api'

export function useSchedulerJobsPage() {
  const schedulerStore = useSchedulerJobsStore()
  const pluginsStore = usePluginsStore()
  const state = storeToRefs(schedulerStore)
  const searchQuery = ref('')
  const statusFilter = ref<'all' | 'success' | 'error'>('all')
  const sortBy = ref<'name' | 'last_run' | 'duration'>('name')
  const now = ref(Date.now())
  let timerId: ReturnType<typeof setInterval> | null = null

  function pluginName(job: SchedulerJobSummary) {
    return pluginsStore.getPluginDisplayName(job.plugin_id, job.plugin_name)
  }

  function getNextRunRelativeText(nextRunTime?: string) {
    return formatNextRunRelative(nextRunTime, now.value)
  }

  async function loadSchedulerJobs() {
    try {
      await schedulerStore.search({
        query: searchQuery.value,
        status: statusFilter.value === 'all' ? undefined : statusFilter.value,
        sort: sortBy.value,
      })
    } catch {
      // The store retains the error for the retry panel.
    }
  }

  async function triggerJob(job: SchedulerJobSummary) {
    try {
      await schedulerStore.trigger(job.job_id)
      notifySuccess(t('scheduler.triggerAccepted'))
    } catch (error) {
      notifyError(getDisplayErrorMessage(error))
    }
  }

  function activatePage() {
    if (timerId !== null) return
    now.value = Date.now()
    void pluginsStore.ensureList().catch(() => undefined)
    schedulerStore.setLiveRefreshActive(true)
    void loadSchedulerJobs()
    timerId = setInterval(() => { now.value = Date.now() }, 10_000)
  }

  function deactivatePage() {
    schedulerStore.setLiveRefreshActive(false)
    if (timerId !== null) clearInterval(timerId)
    timerId = null
  }

  watch([searchQuery, statusFilter, sortBy], () => {
    if (timerId !== null) void loadSchedulerJobs()
  })
  onMounted(activatePage)
  onActivated(activatePage)
  onDeactivated(deactivatePage)
  onUnmounted(deactivatePage)

  return {
    ...state, schedulerStore, pluginName, now,
    searchQuery, statusFilter, sortBy, loadSchedulerJobs, triggerJob, getNextRunRelativeText,
  }
}
