import { defineStore } from 'pinia'

import { createLogsState } from '@/stores/log-state'

export const useLogsStore = defineStore('logs', () => {
  return createLogsState({
    limit: 50,
  })
})
