import { ref } from 'vue'

import type { SchedulerJobSummary } from '@/types/api'

export function useSchedulerJobDetail() {
  const detailVisible = ref(false)
  const currentJob = ref<SchedulerJobSummary | null>(null)

  function showJobDetail(job: SchedulerJobSummary) {
    currentJob.value = job
    detailVisible.value = true
  }

  function closeJobDetail() {
    if (!detailVisible.value) {
      return
    }

    detailVisible.value = false
  }

  function finishJobDetailClose() {
    if (!detailVisible.value) {
      currentJob.value = null
    }
  }

  return {
    closeJobDetail,
    currentJob,
    detailVisible,
    finishJobDetailClose,
    showJobDetail,
  }
}
