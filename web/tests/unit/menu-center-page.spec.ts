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

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
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
    registration_state: 'installed',
    desired_state: 'enabled',
    runtime_state: 'running',
    display_state: 'running',
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
      render_footer: nativeMenuPreviewFooter,
      items: [
        expect.objectContaining({
          name: 'Echo',
          usage: '/help Echo',
        }),
        expect.objectContaining({
          name: 'Weather',
          usage: '/help Weather',
        }),
      ],
    })
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('插件菜单')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('Weather')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('/help Weather')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('template-footer__text')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('Created By RayleaBot 开发版本 &amp; Plugin RayleaBot 开发版本')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('Raylea Footer WenKai')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('lxgw-wenkai-bold')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('../fortune.card/assets/fonts')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('<script')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('<\\/script>')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('</scr${')
    expect(rootPreviewFrame(wrapper).attributes('sandbox')).toBe('allow-same-origin')
    expect(rootPreviewFrame(wrapper).attributes('data-preview-frame-width')).toBe(String(nativePreviewTemplateWidth))
    expect(rootPreviewHost(wrapper).attributes('style')).toContain('--native-template-preview-frame-width: 960px')
    expect(wrapper.text()).toContain('/帮助')

    const pluginSelect = wrapper.getComponent('[data-testid="menu-center-plugin-select"]')
    await pluginSelect.vm.$emit('update:value', 'weather')
    await nextTick()
    expect(pluginPreviewPayload(wrapper)).toMatchObject({
      title: 'Weather',
      subtitle: '天气菜单',
      render_footer: nativeMenuPreviewFooter,
      groups: expect.arrayContaining([
        expect.objectContaining({
          title: '命令',
          items: [
            expect.objectContaining({
              name: 'weather',
              usage: '/weather 上海',
            }),
          ],
        }),
      ]),
    })

    const commandSelect = wrapper.getComponent('[data-testid="menu-center-commands"]')
    const prefixSelect = wrapper.getComponent('[data-testid="menu-center-prefixes"]')
    await commandSelect.vm.$emit('update:value', ['menu', '菜单'])
    await prefixSelect.vm.$emit('update:value', ['#'])
    await nextTick()

    expect(wrapper.text()).toContain('#menu')
    expect(wrapper.text()).toContain('#Weather菜单')
    expect(rootPreviewPayload(wrapper)).toMatchObject({
      items: expect.arrayContaining([
        expect.objectContaining({
          name: 'Weather',
          usage: '#menu Weather',
        }),
      ]),
    })
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('#menu Weather')
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
          prefixes: ['#'],
        },
      },
    }))
    expect(fetchMock.mock.calls.some(([input]) => String(input).includes('/api/system/render/preview'))).toBe(false)
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

function rootPreviewHost(wrapper: ReturnType<typeof mount>) {
  return previewHost(wrapper, 'menu-center-root-preview')
}

function previewFrame(wrapper: ReturnType<typeof mount>, testId: string) {
  return wrapper.get(`[data-testid="${testId}"]`).get('[data-testid="native-template-preview-frame"]')
}

function previewHost(wrapper: ReturnType<typeof mount>, testId: string) {
  return wrapper.get(`[data-testid="${testId}"]`)
}
