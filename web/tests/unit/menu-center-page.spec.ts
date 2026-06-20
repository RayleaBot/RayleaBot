import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import {
  calculateNativePreviewLayout,
  nativePreviewTemplateWidth,
  stripHelpMenuPreviewFontImports,
} from '@/components/NativeTemplatePreviewFrame.vue'
import MenuCenterView from '@/views/builtin/MenuCenterView.vue'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import helpMenuStyles from '../../../templates/help.menu/styles.css?raw'
import type { ConfigDocument, ConfigUpdateResponse, PluginSummary } from '@/types/api'

const nativeMenuPreviewFooter = 'Created By RayleaBot 开发版本 & Plugin RayleaBot 开发版本'
const weatherMenuPreviewFooter = 'Created By RayleaBot 开发版本 & Plugin Weather 1.2.3'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function createConfig(): ConfigDocument {
  return {
    schema_version: '2',
    command: { prefixes: ['/'] },
    builtin_features: {
      menu: {
        commands: ['help', '帮助'],
        prefixes: [],
      },
    },
  } as ConfigDocument
}

function createPlugin(overrides: Partial<PluginSummary>): PluginSummary {
  return {
    id: 'weather',
    name: 'Weather',
    role: 'user',
        state: 'running',
    source: {
      root: 'plugins/installed',
      verified: false,
    },
    trust: {
      level: 'third_party',
      label: '第三方',
    },
    commands: [],
    help: {
      groups: [],
    },
    command_conflicts: [],
    ...overrides,
  } as PluginSummary
}

describe('MenuCenterView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('updates native preview payloads and saves builtin menu config', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const configStore = useConfigStore()
    const pluginsStore = usePluginsStore()
    configStore.document = createConfig()
    pluginsStore.items = [
      createPlugin({
        id: 'weather',
        name: 'Weather',
        version: '1.2.3',
        commands: [
          {
            name: 'weather',
            aliases: ['天气'],
            description: '查询天气',
            usage: 'weather 上海',
            permission: 'everyone',
            command_source: 'manifest',
          },
        ],
        help: {
          summary: '天气菜单',
          groups: [
            {
              title: '查询',
              items: [
                {
                  title: '城市天气',
                  description: '查询城市天气',
                  usage: '/weather 上海',
                  command: 'weather',
                  permission: 'everyone',
                },
              ],
            },
          ],
        },
      }),
      createPlugin({
        id: 'raylea.echo',
        name: 'Echo',
        commands: [
          {
            name: 'echo',
            description: '复读收到的内容',
            command_source: 'manifest',
          },
        ],
        help: {
          summary: '复读菜单',
          groups: [],
        },
      }),
    ]

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchList').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockImplementation(async (nextConfig) => {
      configStore.document = nextConfig
      return {
        config: nextConfig,
        apply_effects: {
          applied_now: [],
          reloaded_now: [],
          restart_required_fields: [],
        },
        redacted_fields: [],
        restart_required: false,
      } as ConfigUpdateResponse
    })

    const wrapper = mount(MenuCenterView, {
      global: {
        plugins: [Antd],
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="menu-center-inherited-prefixes"]').text()).toContain('/')
    expect(rootPreviewPayload(wrapper)).toMatchObject({
      title: '插件菜单',
      subtitle: '当前可用插件',
      command_prefixes: ['/'],
      trigger_examples: ['/help Echo', '/Echo帮助'],
      render_footer: nativeMenuPreviewFooter,
      items: [
        expect.objectContaining({
          name: 'Echo',
        }),
        expect.objectContaining({
          name: 'Weather',
        }),
      ],
    })
    expect(rootPreviewPayload(wrapper).items[0]).not.toHaveProperty('usage')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('插件菜单')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('Weather')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('/help Echo')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('/Echo帮助')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('/help Weather')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('template-footer__text')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('Created By RayleaBot 开发版本 &amp; Plugin RayleaBot 开发版本')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('Raylea Footer WenKai')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('lxgw-wenkai-bold')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('../fortune.card/assets/fonts')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('<script')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('<\\/script>')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('</scr${')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('overflow-x: hidden')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('overflow-y: auto')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('scrollbar-color')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('::-webkit-scrollbar-thumb')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('html::-webkit-scrollbar-thumb')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('body::-webkit-scrollbar-thumb')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain(`html, body {
        min-height: 100%;
        width: ${nativePreviewTemplateWidth}px;`)
    expect(rootPreviewFrame(wrapper).attributes('sandbox')).toBe('allow-same-origin')
    expect(rootPreviewFrame(wrapper).attributes('data-preview-frame-width')).toBe(String(nativePreviewTemplateWidth))
    expect(rootPreviewHost(wrapper).attributes('style')).toContain('--native-template-preview-frame-width: 960px')
    expect(wrapper.text()).toContain('/Echo帮助')

    const pluginSelect = wrapper.getComponent('[data-testid="menu-center-plugin-select"]')
    await pluginSelect.vm.$emit('update:value', 'weather')
    await nextTick()
    expect(pluginPreviewPayload(wrapper)).toMatchObject({
      title: 'Weather',
      subtitle: '天气菜单',
      command_prefixes: ['/'],
      render_footer: weatherMenuPreviewFooter,
      groups: [
        expect.objectContaining({
          title: '查询',
          items: [
            expect.objectContaining({
              name: 'weather',
              command_prefixes: ['/'],
              usage_args: '上海',
            }),
          ],
        }),
      ],
    })
    expect(pluginPreviewPayload(wrapper).groups.some((group: { title: string }) => group.title === '命令')).toBe(false)
    expect(pluginPreviewPayload(wrapper)).not.toHaveProperty('trigger_examples')
    expect(pluginPreviewPayload(wrapper).groups[0].items[0]).not.toHaveProperty('usage')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-usage')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-usage__prefix">/</span>')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('weather')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-usage__text')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-usage__args">上海</span>')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('Plugin Weather 1.2.3')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('Plugin RayleaBot 开发版本')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('card__header')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-permission')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('card__footer')

    const commandSelect = wrapper.getComponent('[data-testid="menu-center-commands"]')
    const prefixSelect = wrapper.getComponent('[data-testid="menu-center-prefixes"]')
    await commandSelect.vm.$emit('update:value', ['menu', '菜单'])
    await prefixSelect.vm.$emit('update:value', ['#', '*'])
    await nextTick()

    expect(wrapper.text()).toContain('#menu')
    expect(wrapper.text()).not.toContain('*Weather菜单')
    expect(rootPreviewPayload(wrapper)).toMatchObject({
      command_prefixes: ['#', '*'],
      trigger_examples: ['#menu Echo', '*Echo菜单'],
      items: expect.arrayContaining([
        expect.objectContaining({
          name: 'Weather',
        }),
      ]),
    })
    expect(rootPreviewPayload(wrapper).items[0]).not.toHaveProperty('usage')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('#menu Echo')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('*Echo菜单')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('#menu Weather')
    expect(pluginPreviewPayload(wrapper)).not.toHaveProperty('trigger_examples')
    expect(pluginPreviewPayload(wrapper).command_prefixes).toEqual(['#', '*'])
    expect(pluginPreviewPayload(wrapper).groups[0].items[0]).toMatchObject({
      command_prefixes: ['#', '*'],
      name: 'weather',
      usage_args: '上海',
    })
    expect(pluginPreviewPayload(wrapper).groups[0].title).toBe('查询')
    expect(pluginPreviewPayload(wrapper).groups.some((group: { title: string }) => group.title === '命令')).toBe(false)
    expect(pluginPreviewPayload(wrapper).groups[0].items[0]).not.toHaveProperty('usage')
    const pluginPreviewSrcdoc = pluginPreviewFrame(wrapper).attributes('srcdoc')
    expect(pluginPreviewSrcdoc).toContain('command-usage__prefix-group')
    expect(pluginPreviewSrcdoc).toContain('command-usage__prefix">#</span>')
    expect(pluginPreviewSrcdoc).toContain('command-usage__prefix">*</span>')
    const previewDoc = new DOMParser().parseFromString(pluginPreviewSrcdoc ?? '', 'text/html')
    const weatherUsages = Array.from(previewDoc.querySelectorAll('.command-usage'))
      .filter((usage) => usage.textContent?.includes('weather'))
    expect(weatherUsages.length).toBeGreaterThan(0)
    for (const usage of weatherUsages) {
      expect(usage.querySelectorAll('code')).toHaveLength(1)
      expect(usage.querySelector('.command-usage__prefix-group')?.textContent).toContain('#')
      expect(usage.querySelector('.command-usage__prefix-group')?.textContent).toContain('*')
      expect(usage.querySelectorAll('.command-usage__name')).toHaveLength(1)
      expect(usage.querySelector('.command-usage__args')?.textContent).toBe('上海')
      expect(usage.querySelector('.command-usage__text')?.textContent).toBe('weather 上海')
    }
    expect(pluginPreviewSrcdoc).not.toContain('</span><span class="command-usage__name">weather</span></code><code>')
    expect(pluginPreviewSrcdoc).not.toContain('command-title__prefixes')
    expect(pluginPreviewSrcdoc).not.toContain('#/weather')
    expect(pluginPreviewSrcdoc).not.toContain('*weather')
    expect(wrapper.find('.menu-center-layout').exists()).toBe(true)
    expect(wrapper.findAll('.menu-preview-card')).toHaveLength(2)
    expect(wrapper.find('.menu-preview-item').exists()).toBe(false)
    expect(wrapper.find('.menu-preview-surface').exists()).toBe(false)

    await wrapper.get('[data-testid="menu-center-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledWith(expect.objectContaining({
      builtin_features: {
        menu: {
          commands: ['menu', '菜单'],
          prefixes: ['#', '*'],
        },
      },
    }))
  })

  it('calculates native preview scaling and internal scroll bounds', () => {
    const fitted = calculateNativePreviewLayout({
      containerTop: 80,
      containerWidth: 960,
      contentHeight: 460,
      viewportHeight: 900,
    })
    expect(fitted.scale).toBe(1)
    expect(fitted.previewHeight).toBe(460)
    expect(fitted.frameHeight).toBe(460)
    expect(fitted.isScrollable).toBe(false)

    const narrow = calculateNativePreviewLayout({
      containerTop: 80,
      containerWidth: 480,
      contentHeight: 900,
      viewportHeight: 900,
    })
    expect(narrow.scale).toBe(0.5)
    expect(narrow.previewHeight).toBe(450)
    expect(narrow.frameHeight).toBe(900)
    expect(narrow.isScrollable).toBe(false)

    const longContent = calculateNativePreviewLayout({
      containerTop: 120,
      containerWidth: 480,
      contentHeight: 2000,
      viewportHeight: 720,
    })
    expect(longContent.scale).toBe(0.5)
    expect(longContent.availableHeight).toBe(576)
    expect(longContent.previewHeight).toBe(576)
    expect(longContent.frameHeight).toBe(1152)
    expect(longContent.isScrollable).toBe(true)
  })

  it('strips help menu font imports from the iframe preview styles', () => {
    const preview = stripHelpMenuPreviewFontImports(helpMenuStyles)

    expect(preview).not.toContain('../fortune.card/assets/fonts/lxgwwenkai-medium/result.css')
    expect(preview).not.toContain('../fortune.card/assets/fonts/lxgw-wenkai-medium/result.css')
    expect(preview).toContain('../fortune.card/assets/fonts/lxgw-wenkai-bold/lxgw-wenkai-bold.ttf')
    expect(preview).toContain('--text-base: 16px')
    expect(preview).toContain('--text-3xl:  46px')
    expect(preview).toMatch(/\.command-usage code\s*\{[\s\S]*font-size: 14px/)
  })
})

function rootPreviewPayload(wrapper: ReturnType<typeof mount>) {
  return previewPayload(wrapper, 'menu-center-root-preview')
}

function pluginPreviewPayload(wrapper: ReturnType<typeof mount>) {
  return previewPayload(wrapper, 'menu-center-plugin-preview')
}

function previewPayload(wrapper: ReturnType<typeof mount>, testId: string) {
  return JSON.parse(previewFrame(wrapper, testId).attributes('data-preview-payload') ?? '{}')
}

function rootPreviewFrame(wrapper: ReturnType<typeof mount>) {
  return previewFrame(wrapper, 'menu-center-root-preview')
}

function pluginPreviewFrame(wrapper: ReturnType<typeof mount>) {
  return previewFrame(wrapper, 'menu-center-plugin-preview')
}

function rootPreviewHost(wrapper: ReturnType<typeof mount>) {
  return previewHost(wrapper, 'menu-center-root-preview')
}

function previewFrame(wrapper: ReturnType<typeof mount>, testId: string) {
  return wrapper.get(`[data-testid="${testId}"]`).get('[data-testid="native-template-preview-frame"]')
}

function previewHost(wrapper: ReturnType<typeof mount>, testId: string) {
  return wrapper.get(`[data-testid="${testId}"]`)
}
