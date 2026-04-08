import { defineStore } from 'pinia'

import { ONEBOT11_PROTOCOL } from '@/lib/protocols'
import { createLogsState } from '@/stores/log-state'

export const useProtocolLogsStore = defineStore('protocolLogs', () => {
  return createLogsState({
    protocol: ONEBOT11_PROTOCOL,
    limit: 50,
  })
})
