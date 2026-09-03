import { ref } from 'vue'
import { defineStore } from 'pinia'

export const useAppAvailabilityStore = defineStore('app-availability', () => {
  const isConnectionInterrupted = ref(false)

  function markConnectionInterrupted() {
    isConnectionInterrupted.value = true
  }

  function markConnected() {
    isConnectionInterrupted.value = false
  }

  return {
    isConnectionInterrupted,
    markConnected,
    markConnectionInterrupted,
  }
})
