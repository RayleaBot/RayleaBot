import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { useToastFeedback } from '@/adapter/feedback'
import RenderTemplatesView from '@/views/system/RenderTemplatesView.vue'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import type {
  RenderTemplateDetail,
  RenderTemplatePreviewHTMLResponse,
  RenderTemplateSummary,
} from '@/types/api'

vi.mock('@/adapter/feedback', () => ({
  useToastFeedback: vi.fn(),
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
    name: '测试群组',
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

function createTemplateDetail(templateId = 'help.menu', updatedAt = '2026-04-18T10:30:00Z'): RenderTemplateDetail {
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
                group_nickname: '测试群名片',
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
      preview_data_json: null,
    }
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
    preview_data_json: null,
  }
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
  }
}

function createWideTemplateDetail(): RenderTemplateDetail {
  return {
    ...createTemplateDetail('help.menu'),
    id: 'fortune.card',
    width: 1124,
    height: 1365,
    updated_at: '2026-05-04T10:30:00Z',
  }
}

function createPreviewHTML(templateId: string, title: string): RenderTemplatePreviewHTMLResponse {
  const isWideTemplate = templateId === 'fortune.card' || templateId === 'fortune.stats'
  return {
    template_id: templateId,
    revision_id: `rev_${templateId.replaceAll('.', '_')}`,
    width: isWideTemplate ? 1124 : 960,
    height: isWideTemplate ? 1365 : 640,
    html: `<!doctype html><html><head><link rel="stylesheet" href="https://cdn.example.test/template-font.css"><style>@font-face{font-family:PreviewExternal;src:url("https://cdn.example.test/template-font.woff2")} .surface{min-height:320px;background-image:url("https://cdn.example.test/template-bg.png")}</style></head><body><main class="surface"><h1>${title}</h1><img src="https://cdn.example.test/avatar.png" alt=""></main></body></html>`,
  }
}

function toastMessages() {
  return vi.mocked(useToastFeedback).mock.calls
    .map(([source]) => {
      if (typeof source === 'function') {
        return source()?.message
      }
      return source.value?.message
    })
    .filter((message): message is string => Boolean(message))
}

function createLocalResourcePreviewHTML(templateId: string, title: string): RenderTemplatePreviewHTMLResponse {
  return {
    template_id: templateId,
    revision_id: `rev_${templateId.replaceAll('.', '_')}`,
    width: 960,
    height: 640,
    html: `<!doctype html><html><head><link rel="stylesheet" href="styles/base.css"><style>.surface{background-image:url("assets/shared.png")}.avatar{background:url("assets/shared.png")}</style></head><body><main class="surface"><h1>${title}</h1><img src="assets/shared.png" alt=""></main></body></html>`,
  }
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
    vi.restoreAllMocks()
    vi.mocked(useToastFeedback).mockClear()
    vi.stubGlobal('fetch', vi.fn(async (path: string) => {
      return new Response(new Blob(['asset'], { type: 'text/plain' }), { status: 200 })
    }))
    let blobSequence = 0
    window.URL.createObjectURL = vi.fn(() => `blob:template-asset-${++blobSequence}`)
    window.URL.revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    vi.mocked(useToastFeedback).mockClear()
  })

  it('renders realtime HTML in an iframe', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML').mockImplementation(async (templateId, payload) => (
      createPreviewHTML(templateId, String(payload.data.title ?? 'preview'))
    ))
    const { wrapper } = await mountPage()

    await flushPromises()

    expect(wrapper.text()).toContain('模板预览')
    expect(wrapper.text()).toContain('help.menu')
    expect(wrapper.text()).toContain('输入结构')
    expect(wrapper.text()).toContain('实时 HTML 预览')
    expect(wrapper.text()).toContain('宽度 960px · 高度自适应（初始 640px）')
    expect(wrapper.text()).toContain('输入数据合法时同步显示当前 HTML 文档。')
    expect(wrapper.text()).not.toContain('任务 ID')
    expect(wrapper.text()).not.toContain('产物 ID')
    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(1)
    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledWith(
      'help.menu',
      { theme: 'default', data: JSON.parse(HELP_MENU_DEFAULT_PREVIEW_DATA) },
      expect.any(AbortSignal),
    )

    const frame = wrapper.get('[data-testid="render-template-preview-frame"]')
    expect(frame.attributes('sandbox')).toBe('allow-same-origin')
    expect(frame.attributes('srcdoc')).toContain('帮助菜单')
    expect(frame.attributes('srcdoc')).toContain('overflow-x:hidden!important')
    expect(frame.attributes('srcdoc')).toContain('https://cdn.example.test/template-font.css')
    expect(frame.attributes('srcdoc')).toContain('https://cdn.example.test/template-font.woff2')
    expect(frame.attributes('srcdoc')).toContain('https://cdn.example.test/template-bg.png')
    expect(frame.attributes('srcdoc')).toContain('https://cdn.example.test/avatar.png')
    expect(frame.attributes('data-preview-payload')).toContain('帮助菜单')
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).not.toContain('等待可预览的 HTML')
  })

  it('shows local help menu preview before server HTML resolves', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    let resolvePreview: ((value: RenderTemplatePreviewHTMLResponse) => void) | null = null
    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML').mockImplementation(() => (
      new Promise((resolve) => {
        resolvePreview = resolve
      })
    ))

    const { wrapper } = await mountPage()
    await flushPromises()

    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="native-template-preview-frame"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="render-template-preview-frame"]').exists()).toBe(false)

    resolvePreview?.(createPreviewHTML('help.menu', '帮助菜单'))
    await flushPromises()

    expect(wrapper.find('[data-testid="native-template-preview-frame"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="render-template-preview-frame"]').attributes('srcdoc')).toContain('帮助菜单')
  })

  it('scales wide templates without exposing horizontal preview scrollbars', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const wideTemplate = createWideTemplateDetail()

    renderTemplatesStore.items = [wideTemplate]
    renderTemplatesStore.detailById = {
      'fortune.card': wideTemplate,
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(wideTemplate)
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML').mockResolvedValue(createPreviewHTML('fortune.card', '今日运势'))

    const { wrapper } = await mountPage('/render/templates/fortune.card')

    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    const host = wrapper.get('[data-testid="render-template-preview-host"]')
    const frame = wrapper.get('[data-testid="render-template-preview-frame"]')
    expect(frame.attributes('data-preview-frame-width')).toBe('1124')
    expect(host.attributes('style')).toContain('--native-template-preview-frame-width: 1124px')
    expect(frame.attributes('srcdoc')).toContain('overflow-x:hidden!important')
    expect(frame.attributes('srcdoc')).toContain('max-width:1124px!important')
    expect(frame.attributes('srcdoc')).toContain('width:1124px!important')
  })

  it('updates iframe html when JSON changes and blocks invalid JSON locally', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML').mockImplementation(async (templateId, payload) => (
      createPreviewHTML(templateId, String(payload.data.title ?? 'preview'))
    ))

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()
    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(1)

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    await textarea.setValue(HELP_MENU_ALTERNATE_PREVIEW_DATA)
    await flushPromises()
    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="render-template-preview-frame"]').attributes('srcdoc')).toContain('帮助菜单（新）')

    await textarea.setValue('{')
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('JSON 解析失败')
    expect(toastMessages().some((message) => message.startsWith('JSON 解析失败'))).toBe(true)
  })

  it('reuses cached preview HTML immediately while refreshing in the background', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

    const help = createTemplateDetail()
    const status = {
      ...createTemplateDetail('help.menu'),
      id: 'status.panel',
      version: '3',
      updated_at: '2026-05-09T05:58:59Z',
    }
    renderTemplatesStore.items = [createTemplateSummary(), createTemplateSummary('status.panel', '2026-05-09T05:58:59Z')]
    renderTemplatesStore.detailById = {
      'help.menu': help,
      'status.panel': status,
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockImplementation(async (templateId) => (
      templateId === 'status.panel' ? status : help
    ))
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML')
      .mockResolvedValueOnce(createPreviewHTML('help.menu', '帮助菜单'))
      .mockResolvedValueOnce(createPreviewHTML('status.panel', 'Runtime Status'))
      .mockImplementation(() => new Promise(() => {}))

    const { wrapper, router } = await mountPage()
    await flushPromises()
    expect(wrapper.get('[data-testid="render-template-preview-frame"]').attributes('srcdoc')).toContain('帮助菜单')

    await router.replace('/render/templates/status.panel')
    await flushPromises()
    expect(wrapper.get('[data-testid="render-template-preview-frame"]').attributes('srcdoc')).toContain('Runtime Status')

    await router.replace('/render/templates/help.menu')
    await flushPromises()

    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(3)
    expect(wrapper.get('[data-testid="render-template-preview-frame"]').attributes('srcdoc')).toContain('帮助菜单')
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).not.toContain('等待可预览的 HTML')
  })

  it('deduplicates local template assets while rewriting preview HTML', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

    renderTemplatesStore.items = [createTemplateSummary()]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML').mockResolvedValue(createLocalResourcePreviewHTML('help.menu', '帮助菜单'))
    const downloadSpy = vi.spyOn(renderTemplatesStore, 'downloadTemplateAsset').mockImplementation(async (_templateId, path) => {
      if (path === 'styles/base.css') {
        return {
          blob: new Blob(['.surface{border-image:url("../assets/shared.png")}'], { type: 'text/css' }),
          filename: null,
        }
      }
      return {
        blob: new Blob(['image'], { type: 'image/png' }),
        filename: null,
      }
    })

    const { wrapper } = await mountPage()
    await flushPromises()
    await flushPromises()

    expect(downloadSpy).toHaveBeenCalledTimes(2)
    expect(downloadSpy).toHaveBeenCalledWith('help.menu', 'styles/base.css', expect.any(AbortSignal))
    expect(downloadSpy).toHaveBeenCalledWith('help.menu', 'assets/shared.png', expect.any(AbortSignal))
    expect(wrapper.get('[data-testid="render-template-preview-frame"]').attributes('srcdoc')).toContain('blob:template-asset-')

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    await textarea.setValue(HELP_MENU_ALTERNATE_PREVIEW_DATA)
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(2)
    expect(downloadSpy).toHaveBeenCalledTimes(2)
  })

  it('groups templates by source and shows plugin ownership', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

    renderTemplatesStore.items = [
      createTemplateSummary(),
      createPluginTemplateSummary(),
    ]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue(createTemplateDetail())
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML').mockResolvedValue(createPreviewHTML('help.menu', '帮助菜单'))

    const { wrapper } = await mountPage()
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    expect(wrapper.text()).toContain('系统模板')
    expect(wrapper.text()).toContain('插件模板')
    expect(wrapper.text()).toContain('plugin.weather-card.card')
    expect(wrapper.text()).toContain('weather-card')
    expect(wrapper.text()).toContain('card')
  })

  it('seeds unknown templates from input schema examples before previewing', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

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
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML').mockImplementation(async (templateId, payload) => (
      createPreviewHTML(templateId, String(payload.data.title ?? 'preview'))
    ))

    const { wrapper } = await mountPage('/render/templates/leaderboard.list')

    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    const textarea = wrapper.get('textarea[aria-label="输入数据 JSON"]')
    expect((textarea.element as HTMLTextAreaElement).value).toContain('本周发言榜')
    expect((textarea.element as HTMLTextAreaElement).value).toContain('items')
    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledWith(
      'leaderboard.list',
      {
        theme: 'default',
        data: {
          title: '本周发言榜',
          subtitle: '统计周期：2026-05-01 至 2026-05-07',
          value_label: '发言数',
          items: [
            {
              avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=100',
              group_nickname: '测试群名片',
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
      },
      expect.any(AbortSignal),
    )
    expect(wrapper.get('[data-testid="render-template-preview-frame"]').attributes('srcdoc')).toContain('本周发言榜')
  })

  it('resubmits unchanged preview data after reloading a changed template', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

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
    vi.spyOn(renderTemplatesStore, 'previewTemplateHTML').mockResolvedValue(createPreviewHTML('help.menu', '帮助菜单'))

    const { wrapper } = await mountPage()

    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(1)

    const reloadButton = wrapper.findAll('button').find((button) => button.text().includes('重新加载当前模板'))
    expect(reloadButton).toBeTruthy()
    await reloadButton!.trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()

    expect(renderTemplatesStore.fetchTemplateWorkspace).toHaveBeenCalledWith('help.menu')
    expect(renderTemplatesStore.previewTemplateHTML).toHaveBeenCalledTimes(2)
  })
})
