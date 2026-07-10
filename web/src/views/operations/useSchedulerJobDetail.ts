import { nextTick, onBeforeUnmount, ref } from 'vue'

import type { SchedulerJobSummary } from '@/types/api'

const MODAL_ANIMATION_MS = 150

function cryptoRandomSuffix(length = 6) {
  const bytes = new Uint8Array(length)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => (byte % 36).toString(36)).join('')
}

export function useSchedulerJobDetail() {
  const detailVisible = ref(false)
  const currentJob = ref<SchedulerJobSummary | null>(null)
  const modalContentReady = ref(false)
  const detailCardRef = ref<HTMLDivElement | null>(null)
  const detailModalTitleId = `scheduler-detail-modal-title-${cryptoRandomSuffix()}`
  let modalReadyTimer: number | null = null

  function clearModalReadyTimer() {
    if (modalReadyTimer !== null) {
      window.clearTimeout(modalReadyTimer)
      modalReadyTimer = null
    }
  }

  function showJobDetail(job: SchedulerJobSummary) {
    currentJob.value = job
    detailVisible.value = true
    clearModalReadyTimer()
    modalReadyTimer = window.setTimeout(() => {
      modalContentReady.value = true
      modalReadyTimer = null
    }, MODAL_ANIMATION_MS)
    void nextTick(() => detailCardRef.value?.focus())
  }

  function closeJobDetail() {
    if (!detailVisible.value) {
      return
    }

    detailVisible.value = false
    clearModalReadyTimer()
    window.setTimeout(() => {
      if (!detailVisible.value) {
        modalContentReady.value = false
        currentJob.value = null
      }
    }, MODAL_ANIMATION_MS)
  }

  onBeforeUnmount(clearModalReadyTimer)

  return {
    closeJobDetail,
    currentJob,
    detailCardRef,
    detailModalTitleId,
    detailVisible,
    modalContentReady,
    showJobDetail,
  }
}
