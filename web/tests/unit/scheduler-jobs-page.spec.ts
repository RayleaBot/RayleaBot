import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { notifySuccess } from '@/adapter/feedback'
import SchedulerJobsPage from '@/views/operations/SchedulerJobsView.vue'
import { useSchedulerJobsStore } from '@/stores/scheduler-jobs'
import { usePluginsStore } from '@/stores/plugins'
import type { SchedulerJobSummary } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({
  notifyError: vi.fn(),
  notifySuccess: vi.fn(),
}))

function makeSchedulerJob(overrides: Partial<SchedulerJobSummary> = {}): SchedulerJobSummary {
  return {
    job_id: 'daily_report',
    plugin_id: 'weather',
    plugin_name: '天气插件',
    task_name: 'daily_report',
    log_label: '每日早报',
    cron_expr: '0 8 * * *',
    timezone: 'Asia/Shanghai',
    enabled: true,
    next_run: '2026-05-26T00:00:00Z',
    last_run: '2026-05-25T00:00:00Z',
    last_duration_ms: 820,
    last_error: {
      code: 'plugin.event_timeout',
      message: 'plugin event response timed out',
      at: '2026-05-24T00:00:00Z',
    },
    payload_summary: {
      conversation_id: 'group:20001',
      target_type: 'group',
      target_id: '20001',
      content: '每日天气推送',
    },
    stats: {
      total: 160,
      success: 150,
      failed: 6,
      timeout: 2,
      retry: 2,
      other: 0,
    },
    ...overrides,
  }
}

describe('SchedulerJobsPage', () => {
  afterEach(() => { vi.useRealTimers() })
  beforeEach(() => {
    setActivePinia(createPinia())
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })) as typeof window.matchMedia
    vi.mocked(notifySuccess).mockReset()
    vi.spyOn(usePluginsStore(), 'ensureList').mockResolvedValue(undefined)
  })

  it('updates the countdown while the job collection stays unchanged and stops refresh on unmount', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
    const store = useSchedulerJobsStore()
    store.items = [makeSchedulerJob({ next_run: '2026-09-23T12:00:25Z' })]
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const setActive = vi.spyOn(store, 'setLiveRefreshActive')
    const wrapper = mount(SchedulerJobsPage, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()

    expect(wrapper.get('.relative-time-pill').text()).toBe('25 秒后')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(wrapper.get('.relative-time-pill').text()).toBe('15 秒后')
    wrapper.unmount()
    expect(setActive).toHaveBeenLastCalledWith(false)
  })


  it('calls an empty filtered result a mismatch and offers to clear the filters', async () => {
    const store = useSchedulerJobsStore()
    const job = makeSchedulerJob()
    store.items = [job]
    // The server answers any search with no jobs, and the unfiltered list with the one job.
    const search = vi.spyOn(store, 'search').mockImplementation(async (query) => { store.items = query?.query ? [] : [job] })
    const wrapper = mount(SchedulerJobsPage, { global: { plugins: [getActivePinia()!] } })
    await flushPromises()

    await wrapper.get('input[aria-label="搜索定时任务"]').setValue('不存在的任务')
    await flushPromises()

    expect(wrapper.text()).toContain('没有符合条件的定时任务')
    expect(wrapper.text()).not.toContain('插件创建定时任务后会显示在这里')
    await wrapper.findAll('button').find(button => button.text() === '清除筛选')!.trigger('click')
    await flushPromises()
    expect(search).toHaveBeenLastCalledWith(expect.objectContaining({ query: '', status: undefined }))
    expect(wrapper.text()).toContain('每日早报')
    wrapper.unmount()
  })

  it('renders scheduler job aggregate state', async () => {
    const store = useSchedulerJobsStore()
    store.items = [makeSchedulerJob()]
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(SchedulerJobsPage, {
      global: {
        plugins: [getActivePinia()!],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('天气插件')
    expect(wrapper.text()).toContain('每日早报')
    expect(wrapper.text()).toContain('group:20001')
    expect(wrapper.text()).toContain('已执行 160 次')
    expect(wrapper.text()).toContain('plugin.event_timeout')
  })

  it('opens a scheduler job detail view without full payload data', async () => {
    const store = useSchedulerJobsStore()
    store.items = [makeSchedulerJob()]
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(SchedulerJobsPage, {
      global: {
        plugins: [getActivePinia()!],
      },
    })
    await flushPromises()

    const viewButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('查看'))
    expect(viewButton).toBeTruthy()
    await viewButton!.trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('天气插件 / weather')
    expect(document.body.textContent).toContain('daily_report / daily_report')
    expect(document.body.textContent).not.toContain('target_type')
  })

  it('triggers a scheduler job and refreshes through the store', async () => {
    const store = useSchedulerJobsStore()
    store.items = [makeSchedulerJob({
      log_label: '',
      last_run: null,
      last_duration_ms: 0,
      payload_summary: {
        conversation_id: '',
        target_type: '',
        target_id: '',
        content: '',
      },
      stats: {
        total: 0,
        success: 0,
        failed: 0,
        timeout: 0,
        retry: 0,
        other: 0,
      },
    })]
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const triggerSpy = vi.spyOn(store, 'trigger').mockResolvedValue({
      job_id: 'daily_report',
      plugin_id: 'weather',
      triggered: true,
    })

    const wrapper = mount(SchedulerJobsPage, {
      global: {
        plugins: [getActivePinia()!],
      },
    })
    await flushPromises()

    const triggerButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('立即执行'))
    expect(triggerButton).toBeTruthy()
    await triggerButton!.trigger('click')
    await flushPromises()

    expect(triggerSpy).toHaveBeenCalledWith('daily_report')
    expect(notifySuccess).toHaveBeenCalledTimes(1)
  })
})
