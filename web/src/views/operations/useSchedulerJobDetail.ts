import { nextTick, ref } from 'vue'

import type { SchedulerJobSummary } from '@/types/api'

function cryptoRandomSuffix(length = 6) {
  const bytes = new Uint8Array(length)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => (byte % 36).toString(36)).join('')
}

export function useSchedulerJobDetail() {
  const detailVisible = ref(false)
  const currentJob = ref<SchedulerJobSummary | null>(null)
  const detailCardRef = ref<HTMLDivElement | null>(null)
  const detailModalTitleId = `scheduler-detail-modal-title-${cryptoRandomSuffix()}`

  function showJobDetail(job: SchedulerJobSummary) {
    currentJob.value = job
    detailVisible.value = true
    void nextTick(() => detailCardRef.value?.focus())
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
    detailCardRef,
    detailModalTitleId,
    detailVisible,
    finishJobDetailClose,
    showJobDetail,
  }
}
