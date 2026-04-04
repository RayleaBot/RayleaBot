import { describe, expect, it } from 'vitest'

import {
  getConnectionChannelLabel,
  getConnectionStatusLabel,
  getPluginDisplayStateLabel,
  getLogLevelLabel,
  getPluginRuntimeStateLabel,
  getSystemStatusLabel,
  getTaskStatusLabel,
  getTaskTypeLabel,
} from '@/lib/display'
import { formatDurationSeconds } from '@/lib/format'

describe('display helpers', () => {
  it('renders chinese labels for connection channels and states', () => {
    expect(getConnectionChannelLabel('events')).toBe('事件流')
    expect(getConnectionStatusLabel('authenticated')).toBe('已认证')
    expect(getConnectionStatusLabel('reconnecting')).toBe('重连中')
  })

  it('renders chinese labels for task, plugin, log, and system states', () => {
    expect(getTaskTypeLabel('render.preview')).toBe('图片预览')
    expect(getTaskStatusLabel('succeeded')).toBe('已完成')
    expect(getPluginRuntimeStateLabel('running')).toBe('运行中')
    expect(getPluginDisplayStateLabel('discovered')).toBe('已识别')
    expect(getLogLevelLabel('warn')).toBe('警告')
    expect(getSystemStatusLabel('shutting_down')).toBe('停止中')
  })

  it('formats durations in chinese units', () => {
    expect(formatDurationSeconds(6)).toBe('6 秒')
    expect(formatDurationSeconds(125)).toBe('2 分钟 5 秒')
    expect(formatDurationSeconds(3660)).toBe('1 小时 1 分钟')
  })
})
