import { describe, expect, it } from 'vitest'

import type { AdapterDescriptor, MessageStatsConnection, MessageStatsResponse } from '@/types/api'
import { arrangeConnections, buildLayers, changeRatio, niceScale, niceScaleForTicks, periodWindow } from '@/views/dashboard/message-stats'

function adapter(id: string, overrides: Partial<AdapterDescriptor> = {}): AdapterDescriptor {
  return { id, protocol: 'onebot11', display_name: id, enabled: true, state: 'connected', summary: '', ...overrides }
}

function connection(id: string, received: number[], sent: number[], overrides: Partial<MessageStatsConnection> = {}): MessageStatsConnection {
  const sum = (values: number[]) => values.reduce((a, b) => a + b, 0)
  return { adapter_id: id, protocol: 'onebot11', configured: true, received, sent, totals: { received: sum(received), sent: sum(sent) }, ...overrides }
}

function stats(connections: MessageStatsConnection[], overrides: Partial<MessageStatsResponse> = {}): MessageStatsResponse {
  const buckets = ['2026-10-01T00:00:00Z', '2026-10-01T01:00:00Z']
  const received = connections.reduce((sum, item) => sum + item.totals.received, 0)
  const sent = connections.reduce((sum, item) => sum + item.totals.sent, 0)
  return {
    granularity: 'hour', timezone: 'Asia/Shanghai', start_at: buckets[0]!, end_at: '2026-10-01T02:00:00Z', as_of: '2026-10-01T01:30:00Z',
    tracking_started_at: '2026-04-02T02:00:00Z', buckets, totals: { received, sent }, connections, incidents: [], incidents_truncated: false,
    ...overrides,
  }
}

describe('periodWindow', () => {
  const now = new Date('2026-10-01T06:30:00Z') // 14:30 in Shanghai

  it('ends every preset at tomorrow\'s local midnight and picks hours or days', () => {
    expect(periodWindow({ kind: 'preset', preset: 'today' }, now, 'Asia/Shanghai'))
      .toEqual({ startAt: '2026-09-30T16:00:00Z', endAt: '2026-10-01T16:00:00Z', granularity: 'hour' })
    expect(periodWindow({ kind: 'preset', preset: 'week' }, now, 'Asia/Shanghai'))
      .toEqual({ startAt: '2026-09-24T16:00:00Z', endAt: '2026-10-01T16:00:00Z', granularity: 'hour' })
    expect(periodWindow({ kind: 'preset', preset: 'month' }, now, 'Asia/Shanghai'))
      .toEqual({ startAt: '2026-09-01T16:00:00Z', endAt: '2026-10-01T16:00:00Z', granularity: 'day' })
  })

  it('reads up to three custom days by the hour and longer ranges by the day', () => {
    expect(periodWindow({ kind: 'custom', from: '2026-09-14', to: '2026-09-16' }, now, 'Asia/Shanghai'))
      .toEqual({ startAt: '2026-09-13T16:00:00Z', endAt: '2026-09-16T16:00:00Z', granularity: 'hour' })
    expect(periodWindow({ kind: 'custom', from: '2026-09-14', to: '2026-09-20' }, now, 'Asia/Shanghai').granularity).toBe('day')
  })

  it('follows daylight saving in the management time zone', () => {
    // New York leaves daylight saving on 1 November 2026, so that local day is 25 hours long.
    const window = periodWindow({ kind: 'custom', from: '2026-11-01', to: '2026-11-01' }, now, 'America/New_York')
    expect(window).toEqual({ startAt: '2026-11-01T04:00:00Z', endAt: '2026-11-02T05:00:00Z', granularity: 'hour' })
  })
})

describe('arrangeConnections', () => {
  it('keeps a connection in trouble in view before busier ones and folds the rest into 其他', () => {
    const adapters = [
      adapter('a'), adapter('b'), adapter('c'),
      adapter('d', { state: 'reconnecting' }),
      adapter('e', { enabled: false, state: 'stopped' }),
    ]
    const response = stats([
      connection('a', [50, 50], [10, 10]),
      connection('b', [30, 30], [5, 5]),
      connection('c', [40, 40], [5, 5]),
      connection('d', [2, 0], [1, 0]),
      connection('e', [9, 0], [1, 0]),
      connection('gone', [7, 0], [1, 0], { configured: false }),
    ])
    const arrangement = arrangeConnections(adapters, response)

    expect(arrangement.shown.map(entry => entry.id)).toEqual(['d', 'a', 'c'])
    expect(arrangement.hidden.map(entry => entry.id)).toEqual(['b', 'e', 'gone'])
    expect(arrangement.hidden.map(entry => entry.stage)).toEqual(['active', 'disabled', 'removed'])
    // Colours follow configuration order among the visible connections, without repeats.
    expect([...arrangement.colorSlots.entries()]).toEqual([['a', 0], ['c', 2], ['d', 3]])
  })

  it('lists up to four connections without folding any', () => {
    const adapters = ['a', 'b', 'c', 'd'].map(id => adapter(id))
    const arrangement = arrangeConnections(adapters, stats(adapters.map(item => connection(item.id, [1, 1], [0, 0]))))
    expect(arrangement.shown).toHaveLength(4)
    expect(arrangement.hidden).toHaveLength(0)
  })

  it('shows configured connections before their counts arrive', () => {
    const arrangement = arrangeConnections([adapter('a'), adapter('b')], null)
    expect(arrangement.shown.map(entry => [entry.id, entry.total, entry.stats])).toEqual([['a', 0, undefined], ['b', 0, undefined]])
  })
})

describe('buildLayers', () => {
  it('stacks the busiest connection at the bottom and 其他 on top, listing them in row order', () => {
    const adapters = ['a', 'b', 'c', 'd', 'e'].map(id => adapter(id))
    const response = stats([
      connection('a', [1, 1], [0, 0]),
      connection('b', [5, 5], [0, 0]),
      connection('c', [3, 3], [0, 0]),
      connection('d', [2, 2], [0, 0]),
      connection('e', [1, 0], [0, 0]),
    ])
    const arrangement = arrangeConnections(adapters, response)
    const { order, stack } = buildLayers(response, arrangement, 'connection')
    expect(order.map(layer => layer.key)).toEqual(['b', 'c', 'd', 'other'])
    expect(stack.map(layer => layer.key)).toEqual(['b', 'c', 'd', 'other'])
    expect(order.at(-1)).toMatchObject({ color: 'other', total: 3, values: [2, 1] })
  })

  it('splits by direction across all connections', () => {
    const response = stats([connection('a', [4, 6], [1, 2]), connection('b', [1, 1], [0, 1])])
    const { order } = buildLayers(response, arrangeConnections([adapter('a'), adapter('b')], response), 'direction')
    expect(order.map(layer => [layer.key, layer.values])).toEqual([['received', [5, 7]], ['sent', [1, 3]]])
  })
})

describe('figures', () => {
  it('picks three to five integer ticks with the smallest top', () => {
    expect(niceScale(130)).toEqual({ max: 150, ticks: 3 })
    expect(niceScale(230)).toEqual({ max: 250, ticks: 5 })
    expect(niceScale(7)).toEqual({ max: 8, ticks: 4 })
    expect(niceScale(0)).toEqual({ max: 3, ticks: 3 })
  })

  it('fits a second axis to the first axis tick count', () => {
    expect(niceScaleForTicks(37, 4)).toEqual({ max: 40, ticks: 4 })
    expect(niceScaleForTicks(41, 4)).toEqual({ max: 80, ticks: 4 })
    expect(niceScaleForTicks(0, 3)).toEqual({ max: 3, ticks: 3 })
  })

  it('has no change without a comparison', () => {
    expect(changeRatio(120, 100)).toBeCloseTo(0.2)
    expect(changeRatio(120, null)).toBeNull()
    expect(changeRatio(120, 0)).toBeNull()
  })
})
