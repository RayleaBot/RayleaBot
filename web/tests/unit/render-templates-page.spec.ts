import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import RenderTemplatesView from '@/views/system/RenderTemplatesView.vue'
import { apiDownload } from '@/lib/http'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import { useSystemStore } from '@/stores/system'
import { useTasksStore } from '@/stores/tasks'
import type { RenderTemplateSummary } from '@/types/api'

vi.mock('@/lib/http', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/http')>()),
  apiDownload: vi.fn(),
}))

const HELP_MENU_DEFAULT_PREVIEW_DATA = JSON.stringify({
  title: '帮助菜单',
  subtitle: '常用命令入口',
  user: {
    avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=100',
    nickname: '星野',
    title: '指令调度员',
    id: '10001',
  },
  group: {
    name: 'RayleaBot 测试群',
  },
  permission: {
    level: 'admin',
  },
  items: [
    {
      name: 'weather',
      description: '查询天气',
      usage: '/weather <城市>',
    },
  ],
}, null, 2)

const HELP_MENU_ALTERNATE_PREVIEW_DATA = JSON.stringify({
  title: '帮助菜单（新）',
}, null, 2)

function createTemplateDetail(templateId = 'help.menu', updatedAt = '2026-04-18T10:30:00Z') {
  if (templateId === 'leaderboard.list') {
    return {
      id: 'leaderboard.list',
      version: '1',
      width: 960,
      height: 420,
      has_input_schema: true,
      updated_at: updatedAt,
      source: {
        type: 'system',
        plugin_id: null,
        local_id: null,
      },
      input_schema_json: {
        type: 'object',
        required: ['title', 'items'],
        properties: {
          title: { type: 'string', description: '排行榜标题' },
          subtitle: { type: 'string', description: '副标题' },
          value_label: { type: 'string', description: '数值列标签' },
          items: {
            type: 'array',
            items: {
              type: 'object',
              required: ['nickname', 'value'],
              properties: {
                avatar_url: { type: 'string' },
                group_nickname: { type: 'string' },
                nickname: { type: 'string' },
                title: { type: 'string' },
                value: { type: ['string', 'number'] },
              },
            },
          },
        },
        examples: [
          {
            title: '本周发言榜',
            subtitle: '统计周期：2026-05-01 至 2026-05-07',
            value_label: '发言数',
            items: [
              {
                avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=100',
                group_nickname: '银蝶',
                nickname: 'Silver',
                title: '群主',
                value: 128,
              },
              {
                nickname: 'Nova',
                value: 81,
              },
            ],
          },
        ],
      },
    } as const
  }

  if (templateId === 'status.panel') {
    return {
      id: 'status.panel',
      version: '1',
      width: 960,
      height: 540,
      has_input_schema: true,
      updated_at: updatedAt,
      source: {
        type: 'system',
        plugin_id: null,
        local_id: null,
      },
      input_schema_json: {
        type: 'object',
        required: ['title', 'status'],
        properties: {
          title: { type: 'string', description: '状态标题' },
          status: { type: 'string', description: '当前状态' },
        },
      },
    } as const
  }

  return {
    id: 'help.menu',
    version: '1',
    width: 960,
    height: 640,
    has_input_schema: true,
    updated_at: updatedAt,
    source: {
      type: 'system',
      plugin_id: null,
      local_id: null,
    },
    input_schema_json: {
      type: 'object',
      properties: {
        title: { type: 'string', description: '主标题' },
        items: { type: 'array', description: '菜单项' },
      },
      required: ['title'],
    },
  } as const
}

function createTemplateSummary(templateId = 'help.menu', updatedAt = '2026-04-18T10:30:00Z'): RenderTemplateSummary {
  const detail = createTemplateDetail(templateId, updatedAt)
  return {
    id: detail.id,
    version: detail.version,
    width: detail.width,
    height: detail.height,
    has_input_schema: detail.has_input_schema,
    updated_at: detail.updated_at,
    source: detail.source,
  }
}

function createPluginTemplateSummary(): RenderTemplateSummary {
  return {
    id: 'plugin.weather-card.card',
    version: '1',
    width: 320,
    height: 240,
    has_input_schema: true,
    updated_at: '2026-04-18T10:29:00Z',
    source: {
      type: 'plugin',
      plugin_id: 'weather-card',
      local_id: 'card',
    },
  } as const
}

function createRouterForPage() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/render/templates/:templateId?',
        name: 'render-templates',
        component: RenderTemplatesView,
      },
      {
        path: '/tasks',
        name: 'tasks',
        component: { template: '<div>tasks</div>' },
      },
    ],
  })
}

async function mountPage(initialPath = '/render/templates/help.menu') {
  const router = createRouterForPage()
  await router.push(initialPath)
  await router.isReady()

  const wrapper = mount(RenderTemplatesView, {
    global: {
      plugins: [Antd, router],
    },
  })

  await flushPromises()

  return { wrapper, router }
}

describe('RenderTemplatesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    window.URL.createObjectURL = vi.fn(() => 'blob:render-preview')
    window.URL.revokeObjectURL = vi.fn()
    vi.mocked(apiDownload).mockReset()
    vi.mocked(apiDownload).mockResolvedValue({
      blob: new Blob(['preview'], { type: 'image/png' }),
      filename: null,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the template preview workspace and auto-submits preview requests', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(systemStore, 'previewRender').mockResolvedValue({ task_id: 'task_render_preview_0001' })
    vi.spyOn(tasksStore, 'fetchTask').mockImplementation(async () => {
      tasksStore.items = [
        {
          task_id: 'task_render_preview_0001',
          task_type: 'render.preview',
          status: 'succeeded',
          summary: 'render preview for help.menu',
          result: {
            summary: 'render preview complete',
            details: {
              artifact_id: 'render_preview_0001.png',
              image_url: '/api/system/render/artifacts/render_preview_0001.png',
              from_cache: false,
            },
          },
        },
      ]
      return tasksStore.items[0]!
    })

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(wrapper.text()).toContain('模板预览')
    expect(wrapper.text()).toContain('help.menu')
    expect(wrapper.text()).toContain('输入结构')
    expect(wrapper.text()).toContain('渲染参数')
    expect(wrapper.text()).toContain('宽度 960px · 高度自适应（初始 640px）')
    expect(wrapper.text()).toContain('按内容高度生成最新预览')
    expect(wrapper.text()).not.toContain('画布尺寸')
    expect(wrapper.text()).not.toContain('960 × 640')
    expect(wrapper.text()).not.toContain('主题')
    expect(wrapper.text()).not.toContain('输出格式')
    expect(wrapper.text()).not.toContain('源码编辑')
    expect(wrapper.text()).not.toContain('版本历史')
    expect(wrapper.text()).not.toContain('执行校验')
    expect(wrapper.text()).not.toContain('保存模板')
    expect(systemStore.previewRender).toHaveBeenCalledTimes(1)
    const previewPayload = vi.mocked(systemStore.previewRender).mock.calls[0]![0]
    expect(previewPayload).toEqual({
      template: 'help.menu',
      data: JSON.parse(HELP_MENU_DEFAULT_PREVIEW_DATA),
    })
    expect(previewPayload).not.toHaveProperty('theme')
    expect(previewPayload).not.toHaveProperty('output')
    expect(systemStore.previewRender).not.toHaveBeenCalledWith(expect.objectContaining({
      draft: expect.anything(),
    }))
    expect(tasksStore.fetchTask).toHaveBeenCalledWith('task_render_preview_0001', { makeCurrent: false })
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).toContain('render_preview_0001.png')
    expect(wrapper.find('img[alt="模板预览结果"]').attributes('src')).toBe('blob:render-preview')
  })

  it('groups templates by source and shows plugin ownership', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [
      createTemplateSummary(),
      createPluginTemplateSummary(),
    ]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(systemStore, 'previewRender').mockResolvedValue({ task_id: 'task_render_preview_0001' })
    vi.spyOn(tasksStore, 'fetchTask').mockResolvedValue({
      task_id: 'task_render_preview_0001',
      task_type: 'render.preview',
      status: 'pending',
      summary: 'render preview for help.menu',
    })

    const { wrapper } = await mountPage()

    expect(wrapper.text()).toContain('系统模板')
    expect(wrapper.text()).toContain('插件模板')
    expect(wrapper.text()).toContain('plugin.weather-card.card')
    expect(wrapper.text()).toContain('weather-card')
    expect(wrapper.text()).toContain('card')
  })

  it('does not resubmit identical payloads and blocks invalid preview json', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(systemStore, 'previewRender').mockResolvedValue({ task_id: 'task_render_preview_0001' })
    vi.spyOn(tasksStore, 'fetchTask').mockImplementation(async () => {
      tasksStore.items = [
        {
          task_id: 'task_render_preview_0001',
          task_type: 'render.preview',
          status: 'succeeded',
          summary: 'render preview for help.menu',
          result: {
            summary: 'render preview complete',
            details: {
              artifact_id: 'render_preview_0001.png',
              image_url: '/api/system/render/artifacts/render_preview_0001.png',
              from_cache: false,
            },
          },
        },
      ]
      return tasksStore.items[0]!
    })

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    await textarea.setValue(HELP_MENU_DEFAULT_PREVIEW_DATA)
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(systemStore.previewRender).toHaveBeenCalledTimes(1)

    await textarea.setValue('{')
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(systemStore.previewRender).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('JSON 解析失败')
  })

  it('seeds unknown templates from input schema examples before previewing', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [createTemplateSummary('leaderboard.list', '2026-05-03T01:01:04Z')]

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockImplementation(async () => {
      const detail = createTemplateDetail('leaderboard.list', '2026-05-03T01:01:04Z')
      renderTemplatesStore.detailById = {
        ...renderTemplatesStore.detailById,
        'leaderboard.list': detail,
      }
      renderTemplatesStore.items = [detail]
      return detail
    })
    vi.spyOn(systemStore, 'previewRender').mockResolvedValue({ task_id: 'task_render_preview_leaderboard' })
    vi.spyOn(tasksStore, 'fetchTask').mockImplementation(async () => {
      tasksStore.items = [
        {
          task_id: 'task_render_preview_leaderboard',
          task_type: 'render.preview',
          status: 'succeeded',
          summary: 'render preview for leaderboard.list',
          result: {
            summary: 'render preview complete',
            details: {
              artifact_id: 'render_preview_leaderboard.png',
              image_url: '/api/system/render/artifacts/render_preview_leaderboard.png',
              from_cache: false,
            },
          },
        },
      ]
      return tasksStore.items[0]!
    })

    const { wrapper } = await mountPage('/render/templates/leaderboard.list')

    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    expect((textarea.element as HTMLTextAreaElement).value).toContain('本周发言榜')
    expect((textarea.element as HTMLTextAreaElement).value).toContain('items')
    expect(systemStore.previewRender).toHaveBeenCalledTimes(1)
    expect(vi.mocked(systemStore.previewRender).mock.calls[0]![0]).toEqual({
      template: 'leaderboard.list',
      data: {
        title: '本周发言榜',
        subtitle: '统计周期：2026-05-01 至 2026-05-07',
        value_label: '发言数',
        items: [
          {
            avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=100',
            group_nickname: '银蝶',
            nickname: 'Silver',
            title: '群主',
            value: 128,
          },
          {
            nickname: 'Nova',
            value: 81,
          },
        ],
      },
    })
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).toContain('render_preview_leaderboard.png')
  })

  it('resubmits unchanged preview data after reloading a changed template', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockImplementation(async () => {
      const detail = createTemplateDetail('help.menu', '2026-04-18T10:35:00Z')
      renderTemplatesStore.detailById = {
        ...renderTemplatesStore.detailById,
        'help.menu': detail,
      }
      renderTemplatesStore.items = [detail]
      return detail
    })
    vi.spyOn(systemStore, 'previewRender')
      .mockResolvedValueOnce({ task_id: 'task_render_preview_0001' })
      .mockResolvedValueOnce({ task_id: 'task_render_preview_0002' })
    vi.spyOn(tasksStore, 'fetchTask').mockImplementation(async (taskId) => {
      const artifactId = `${taskId}.png`
      tasksStore.items = [
        {
          task_id: taskId,
          task_type: 'render.preview',
          status: 'succeeded',
          summary: `render preview for ${taskId}`,
          result: {
            summary: 'render preview complete',
            details: {
              artifact_id: artifactId,
              image_url: `/api/system/render/artifacts/${artifactId}`,
              from_cache: false,
            },
          },
        },
      ]
      return tasksStore.items[0]!
    })

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(systemStore.previewRender).toHaveBeenCalledTimes(1)

    const reloadButton = wrapper.findAll('button').find((button) => button.text().includes('重新加载当前模板'))
    expect(reloadButton).toBeTruthy()
    await reloadButton!.trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(renderTemplatesStore.fetchTemplateWorkspace).toHaveBeenCalledWith('help.menu')
    expect(systemStore.previewRender).toHaveBeenCalledTimes(2)
    expect(vi.mocked(systemStore.previewRender).mock.calls[1]![0]).toEqual({
      template: 'help.menu',
      data: JSON.parse(HELP_MENU_DEFAULT_PREVIEW_DATA),
    })
  })

  it('keeps the current template in pending state while a newer preview request is still running', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())

    let resolveFirst: ((value: { task_id: string }) => void) | null = null
    let resolveSecond: ((value: { task_id: string }) => void) | null = null
    vi.spyOn(systemStore, 'previewRender')
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveFirst = resolve
      }))
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveSecond = resolve
      }))

    vi.spyOn(tasksStore, 'fetchTask').mockImplementation(async () => {
      tasksStore.items = [
        {
          task_id: 'task_render_preview_0002',
          task_type: 'render.preview',
          status: 'succeeded',
          summary: 'render preview for help.menu',
          result: {
            summary: 'render preview complete',
            details: {
              artifact_id: 'render_preview_0002.png',
              image_url: '/api/system/render/artifacts/render_preview_0002.png',
              from_cache: false,
            },
          },
        },
      ]
      return tasksStore.items[0]!
    })

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    await textarea.setValue(HELP_MENU_ALTERNATE_PREVIEW_DATA)
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(systemStore.previewRender).toHaveBeenCalledTimes(2)

    resolveFirst?.({ task_id: 'task_render_preview_0001' })
    await flushPromises()

    const previewResult = wrapper.get('[data-testid="render-template-preview-result"]')
    expect(previewResult.text()).toContain('正在生成最新预览。')
    expect(wrapper.text()).not.toContain('等待生成预览。')

    resolveSecond?.({ task_id: 'task_render_preview_0002' })
    await flushPromises()
  })

  it('does not resubmit an already pending payload after reverting to it', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())

    let resolveFirst: ((value: { task_id: string }) => void) | null = null
    let resolveSecond: ((value: { task_id: string }) => void) | null = null
    vi.spyOn(systemStore, 'previewRender')
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveFirst = resolve
      }))
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveSecond = resolve
      }))

    vi.spyOn(tasksStore, 'fetchTask').mockImplementation(async (taskId) => {
      const artifactId = taskId === 'task_render_preview_0002' ? 'render_preview_0002.png' : 'render_preview_0001.png'
      tasksStore.items = [
        {
          task_id: taskId,
          task_type: 'render.preview',
          status: 'succeeded',
          summary: `render preview for ${taskId}`,
          result: {
            summary: 'render preview complete',
            details: {
              artifact_id: artifactId,
              image_url: `/api/system/render/artifacts/${artifactId}`,
              from_cache: false,
            },
          },
        },
      ]
      return tasksStore.items[0]!
    })

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    await textarea.setValue(HELP_MENU_ALTERNATE_PREVIEW_DATA)
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    await textarea.setValue(HELP_MENU_DEFAULT_PREVIEW_DATA)
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(systemStore.previewRender).toHaveBeenCalledTimes(2)

    resolveFirst?.({ task_id: 'task_render_preview_0001' })
    await flushPromises()
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).toContain('render_preview_0001.png')
    expect(wrapper.text()).not.toContain('正在生成最新预览。')

    resolveSecond?.({ task_id: 'task_render_preview_0002' })
    await flushPromises()
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).toContain('render_preview_0001.png')
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).not.toContain('render_preview_0002.png')
  })

  it('clears stale preview errors when the inputs return to the last successful payload', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(systemStore, 'previewRender')
      .mockResolvedValueOnce({ task_id: 'task_render_preview_0001' })
      .mockRejectedValueOnce(new Error('服务繁忙'))
    vi.spyOn(tasksStore, 'fetchTask').mockImplementation(async () => {
      tasksStore.items = [
        {
          task_id: 'task_render_preview_0001',
          task_type: 'render.preview',
          status: 'succeeded',
          summary: 'render preview for help.menu',
          result: {
            summary: 'render preview complete',
            details: {
              artifact_id: 'render_preview_0001.png',
              image_url: '/api/system/render/artifacts/render_preview_0001.png',
              from_cache: false,
            },
          },
        },
      ]
      return tasksStore.items[0]!
    })

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    await textarea.setValue(HELP_MENU_ALTERNATE_PREVIEW_DATA)
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(wrapper.text()).toContain('服务繁忙')

    await textarea.setValue(HELP_MENU_DEFAULT_PREVIEW_DATA)
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    expect(systemStore.previewRender).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('服务繁忙')
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).toContain('render_preview_0001.png')
  })

  it('keeps the latest accepted preview when an older request resolves later', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())

    let resolveFirst: ((value: { task_id: string }) => void) | null = null
    let resolveSecond: ((value: { task_id: string }) => void) | null = null
    vi.spyOn(systemStore, 'previewRender')
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveFirst = resolve
      }))
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveSecond = resolve
      }))

    vi.spyOn(tasksStore, 'fetchTask').mockImplementation(async (taskId) => {
      const artifactId = taskId === 'task_render_preview_0002' ? 'render_preview_0002.png' : 'render_preview_0001.png'
      tasksStore.items = [
        {
          task_id: taskId,
          task_type: 'render.preview',
          status: 'succeeded',
          summary: `render preview for ${taskId}`,
          result: {
            summary: 'render preview complete',
            details: {
              artifact_id: artifactId,
              image_url: `/api/system/render/artifacts/${artifactId}`,
              from_cache: false,
            },
          },
        },
      ]
      return tasksStore.items[0]!
    })

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    await textarea.setValue(HELP_MENU_ALTERNATE_PREVIEW_DATA)
    await vi.advanceTimersByTimeAsync(500)
    await flushPromises()

    resolveSecond?.({ task_id: 'task_render_preview_0002' })
    await flushPromises()
    resolveFirst?.({ task_id: 'task_render_preview_0001' })
    await flushPromises()

    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).toContain('render_preview_0002.png')
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).not.toContain('render_preview_0001.png')
  })

  it('auto-selects the first template once and does not pull other routes back', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

    renderTemplatesStore.items = [
      createTemplateSummary(),
      createTemplateSummary('status.panel', '2026-04-18T10:31:00Z'),
    ]

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())

    const { router } = await mountPage('/render/templates')

    await flushPromises()

    expect(router.currentRoute.value.fullPath).toBe('/render/templates/help.menu')

    await router.push('/tasks')
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toBe('/tasks')
  })
})
