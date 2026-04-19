import Antd from 'ant-design-vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'

import ManagementLogDetailDrawer from '@/components/logs/ManagementLogDetailDrawer.vue'
import { clearLogDetailWindowPositionMemory } from '@/components/logs/log-detail-window-memory'
import type { LogDetailResponse, LogSummary } from '@/types/api'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

class PointerEventMock extends MouseEvent {
  pointerId: number

  constructor(type: string, init: MouseEventInit & { pointerId?: number } = {}) {
    super(type, init)
    this.pointerId = init.pointerId ?? 1
  }
}

function createRect(width: number, height: number, left = 0, top = 0): DOMRect {
  return {
    x: left,
    y: top,
    width,
    height,
    left,
    top,
    right: left + width,
    bottom: top + height,
    toJSON() {
      return {}
    },
  } as DOMRect
}

function mockElementRect(element: Element, width: number, height: number, left = 0, top = 0) {
  Object.defineProperty(element, 'getBoundingClientRect', {
    configurable: true,
    value: () => createRect(width, height, left, top),
  })
}

function attachDynamicPanelRect(panel: HTMLElement, hostLeft = 0, hostTop = 0) {
  Object.defineProperty(panel, 'getBoundingClientRect', {
    configurable: true,
    value: () => createRect(
      Number.parseFloat(panel.style.width || '0') || 0,
      Number.parseFloat(panel.style.height || '0') || 0,
      hostLeft + (Number.parseFloat(panel.style.left || '0') || 0),
      hostTop + (Number.parseFloat(panel.style.top || '0') || 0),
    ),
  })
}

function createSummary(overrides: Partial<LogSummary> = {}): LogSummary {
  return {
    log_id: 'log_detail_0001',
    timestamp: '2026-04-17T10:16:00Z',
    level: 'warn',
    source: 'adapter.onebot11',
    protocol: 'onebot11',
    plugin_id: 'weather',
    request_id: 'req_log_detail_0001',
    message: 'ignored OneBot API response with unsupported echo',
    ...overrides,
  }
}

function createDetail(overrides: Partial<LogDetailResponse> = {}): LogDetailResponse {
  return {
    log_id: 'log_detail_0001',
    timestamp: '2026-04-17T10:16:00Z',
    level: 'warn',
    source: 'adapter.onebot11',
    protocol: 'onebot11',
    plugin_id: 'weather',
    request_id: 'req_log_detail_0001',
    message: 'ignored OneBot API response with unsupported echo',
    details: {
      direction: 'inbound',
      frame_type: 'api.response.ignored',
      reason: 'echo must be a non-empty string',
    },
    ...overrides,
  }
}

async function mountFloatingDrawer(options: {
  memoryKey: string
  hostWidth?: number
  hostHeight?: number
  summary?: LogSummary
  detail?: LogDetailResponse | null
  open?: boolean
  scope?: 'current_session' | 'history'
}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/logs', name: 'logs', component: { template: '<div>logs</div>' } },
      { path: '/logs/history', name: 'logs-history', component: { template: '<div>history</div>' } },
      { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
      { path: '/protocols', name: 'protocols', component: { template: '<div>protocols</div>' } },
    ],
  })
  await router.push(options.scope === 'history' ? '/logs/history' : '/logs')
  await router.isReady()

  const hostElement = document.createElement('div')
  document.body.appendChild(hostElement)
  mockElementRect(hostElement, options.hostWidth ?? 1600, options.hostHeight ?? 900)

  const wrapper = mount(ManagementLogDetailDrawer, {
    attachTo: document.body,
    props: {
      open: options.open ?? true,
      loading: false,
      error: null,
      summary: options.summary ?? createSummary(),
      detail: options.detail ?? createDetail(),
      scope: options.scope ?? 'current_session',
      memoryKey: options.memoryKey,
      hostElement,
    },
    global: {
      plugins: [Antd, router],
    },
  })

  await flushPromises()
  await nextTick()
  mountedWrappers.push(wrapper)

  return {
    wrapper,
    hostElement,
  }
}

function readWindowPosition(wrapper: ReturnType<typeof mount>) {
  const panel = wrapper.get('.log-detail-window').element as HTMLElement
  return {
    left: Number.parseFloat(panel.style.left || '0') || 0,
    top: Number.parseFloat(panel.style.top || '0') || 0,
  }
}

async function dragWindow(wrapper: ReturnType<typeof mount>, target: { clientX: number, clientY: number }) {
  const panel = wrapper.get('.log-detail-window').element as HTMLElement
  attachDynamicPanelRect(panel)

  const header = wrapper.get('.log-detail-window__header').element as HTMLElement
  header.dispatchEvent(new PointerEventMock('pointerdown', {
    bubbles: true,
    button: 0,
    clientX: 1500,
    clientY: 36,
    pointerId: 7,
  }))
  window.dispatchEvent(new PointerEventMock('pointermove', {
    bubbles: true,
    clientX: target.clientX,
    clientY: target.clientY,
    pointerId: 7,
  }))
  window.dispatchEvent(new PointerEventMock('pointerup', {
    bubbles: true,
    clientX: target.clientX,
    clientY: target.clientY,
    pointerId: 7,
  }))

  await nextTick()
}

describe('ManagementLogDetailDrawer', () => {
  beforeEach(() => {
    clearLogDetailWindowPositionMemory()
    document.body.innerHTML = ''
    vi.stubGlobal('PointerEvent', PointerEventMock)
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    document.body.innerHTML = ''
  })

  it('renders a floating window on desktop hosts', async () => {
    const { wrapper } = await mountFloatingDrawer({
      memoryKey: 'logs-current',
    })

    expect(wrapper.find('.log-detail-window').exists()).toBe(true)
    expect(wrapper.find('.ant-drawer').exists()).toBe(false)
    expect(wrapper.text()).toContain('日志详情')
    expect(wrapper.text()).toContain('详情 JSON')
    expect(wrapper.text()).toContain('weather')
    expect(wrapper.text()).toContain('相关实时日志')
  })

  it('falls back to the drawer on narrow screens', async () => {
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query.includes('max-width: 960px'),
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })) as typeof window.matchMedia

    const { wrapper } = await mountFloatingDrawer({
      memoryKey: 'logs-current',
    })

    expect(wrapper.find('.ant-drawer').exists()).toBe(true)
    expect(wrapper.find('.log-detail-window').exists()).toBe(false)
  })

  it('keeps dragging inside the right corridor and remembers the last position', async () => {
    const { wrapper } = await mountFloatingDrawer({
      memoryKey: 'logs-current',
      hostWidth: 1600,
      hostHeight: 900,
    })

    await dragWindow(wrapper, {
      clientX: 100,
      clientY: 1500,
    })

    const firstPosition = readWindowPosition(wrapper)
    expect(firstPosition.left).toBe(836)
    expect(firstPosition.top).toBe(28)

    await wrapper.setProps({ open: false })
    await flushPromises()
    await wrapper.setProps({ open: true })
    await flushPromises()

    const reopenedPosition = readWindowPosition(wrapper)
    expect(reopenedPosition).toEqual(firstPosition)
  })

  it('keeps positions isolated by memory key', async () => {
    const current = await mountFloatingDrawer({
      memoryKey: 'logs-current',
      hostWidth: 1600,
      hostHeight: 900,
    })

    await dragWindow(current.wrapper, {
      clientX: 100,
      clientY: 300,
    })
    const currentPosition = readWindowPosition(current.wrapper)
    expect(currentPosition.left).toBe(836)
    current.wrapper.unmount()

    const history = await mountFloatingDrawer({
      memoryKey: 'logs-history',
      hostWidth: 1600,
      hostHeight: 900,
    })
    const historyPosition = readWindowPosition(history.wrapper)
    expect(historyPosition.left).toBe(908)
    history.wrapper.unmount()

    const reopenedCurrent = await mountFloatingDrawer({
      memoryKey: 'logs-current',
      hostWidth: 1600,
      hostHeight: 900,
    })
    expect(readWindowPosition(reopenedCurrent.wrapper)).toEqual(currentPosition)
  })

  it('switches request links with the log scope', async () => {
    const current = await mountFloatingDrawer({
      memoryKey: 'logs-current',
      scope: 'current_session',
    })
    expect(current.wrapper.text()).toContain('相关实时日志')
    current.wrapper.unmount()

    const history = await mountFloatingDrawer({
      memoryKey: 'logs-history',
      scope: 'history',
    })
    expect(history.wrapper.text()).toContain('相关历史日志')
  })

  it('updates the content in place when a different log is selected', async () => {
    const { wrapper } = await mountFloatingDrawer({
      memoryKey: 'logs-current',
      hostWidth: 1600,
      hostHeight: 900,
    })

    await dragWindow(wrapper, {
      clientX: 100,
      clientY: 300,
    })

    const beforeSwitch = readWindowPosition(wrapper)
    await wrapper.setProps({
      summary: createSummary({
        log_id: 'log_detail_0002',
        source: 'runtime',
        message: 'runtime switched to the next log row',
      }),
      detail: createDetail({
        log_id: 'log_detail_0002',
        source: 'runtime',
        message: 'runtime switched to the next log row',
        details: {
          branch: 'history',
        },
      }),
    })
    await flushPromises()

    expect(readWindowPosition(wrapper)).toEqual(beforeSwitch)
    expect(wrapper.text()).toContain('runtime switched to the next log row')
    expect(wrapper.text()).toContain('runtime')
  })
})
