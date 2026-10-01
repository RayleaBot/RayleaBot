import { ref } from 'vue'
import { defineStore } from 'pinia'

// The server announces over /ws/events that message counts or connection outages changed. The notice carries no
// counts, so whoever shows them rereads them when it arrives.
export const useMessageStatsLiveStore = defineStore('message-stats-live', () => {
  const changedAt = ref(0)

  function notifyChanged() {
    changedAt.value = Date.now()
  }

  return { changedAt, notifyChanged }
})
