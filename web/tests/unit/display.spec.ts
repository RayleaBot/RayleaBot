import { describe, expect, it } from 'vitest'

import {
  getConnectionChannelLabel,
  getConnectionStatusLabel,
  getLogLevelLabel,
  getPluginStateLabel,
  getSystemStatusLabel,
} from '@/lib/display'
import { formatDurationSeconds } from '@/lib/format'

describe('display helpers', () => {
  it('renders chinese labels for connection channels and states', () => {
    expect(getConnectionChannelLabel('events')).toBe('事件流')
    expect(getConnectionStatusLabel('authenticated')).toBe('已认证')
    expect(getConnectionStatusLabel('reconnecting')).toBe('重连中')
  })

  it('renders chinese labels for plugin, log, and system states', () => {
    expect(getPluginStateLabel('running')).toBe('运行中')
    expect(getPluginStateLabel('invalid')).toBe('清单异常')
    expect(getLogLevelLabel('warn')).toBe('warn')
    expect(getSystemStatusLabel('shutting_down')).toBe('停止中')
  })

  it('formats durations in chinese units', () => {
    expect(formatDurationSeconds(6)).toBe('6 秒')
    expect(formatDurationSeconds(125)).toBe('2 分钟 5 秒')
    expect(formatDurationSeconds(3660)).toBe('1 小时 1 分钟')
  })
})
