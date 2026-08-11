import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import NativeTemplatePreviewFrame, {
  calculateNativePreviewLayout,
  nativePreviewTemplateWidth,
  stripHelpMenuPreviewFontImports,
} from '@/components/NativeTemplatePreviewFrame.vue'
import MenuCenterView from '@/views/builtin/MenuCenterView.vue'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import helpMenuStyles from '../../../templates/help.menu/styles.css?raw'
import helpMenuTemplate from '../../../templates/help.menu/template.html?raw'
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
    role: 'community',
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
            usage: '/weather [空气质量] 城市',
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
                  usage: '/weather [空气质量] 城市',
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
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-prefixes__label">前缀</span>')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('command-guide__block--prefixes')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('template-footer__text')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('Created By RayleaBot 开发版本 &amp; Plugin RayleaBot 开发版本')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).toContain('LXGW WenKai Medium')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toMatch(/\.ttf\b/i)
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('../fortune.card/assets/fonts')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('<script')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('<\\/script>')
    expect(rootPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('</scr${')
    expect(rootPreviewFrame(wrapper).attributes('sandbox')).toBe('allow-same-origin')
    expect(rootPreviewFrame(wrapper).attributes('data-preview-frame-width')).toBe(String(nativePreviewTemplateWidth))
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
              usage_args: '[空气质量] 城市',
              usage_parts: [
                { kind: 'optional', text: '空气质量' },
                { kind: 'required', text: '城市' },
              ],
            }),
          ],
        }),
      ],
    })
    expect(pluginPreviewPayload(wrapper).groups.some((group: { title: string }) => group.title === '命令')).toBe(false)
    expect(pluginPreviewPayload(wrapper)).not.toHaveProperty('trigger_examples')
    expect(pluginPreviewPayload(wrapper).groups[0].items[0]).not.toHaveProperty('usage')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-usage')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-prefixes__label">前缀</span>')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('class="command-prefix-cue"')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('command-usage__prefix')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('weather')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-argument--optional')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-argument--required')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('Plugin Weather 1.2.3')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('Plugin RayleaBot 开发版本')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('cell__header')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).toContain('command-permission')
    expect(pluginPreviewFrame(wrapper).attributes('srcdoc')).not.toContain('cell__footer')

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
      usage_args: '[空气质量] 城市',
      usage_parts: [
        { kind: 'optional', text: '空气质量' },
        { kind: 'required', text: '城市' },
      ],
    })
    expect(pluginPreviewPayload(wrapper).groups[0].title).toBe('查询')
    expect(pluginPreviewPayload(wrapper).groups.some((group: { title: string }) => group.title === '命令')).toBe(false)
    expect(pluginPreviewPayload(wrapper).groups[0].items[0]).not.toHaveProperty('usage')
    const pluginPreviewSrcdoc = pluginPreviewFrame(wrapper).attributes('srcdoc')
    expect(pluginPreviewSrcdoc).not.toContain('command-usage__prefix')
    const previewDoc = new DOMParser().parseFromString(pluginPreviewSrcdoc ?? '', 'text/html')
    expect(previewDoc.querySelector('.command-prefixes__values')?.textContent).toContain('#')
    expect(previewDoc.querySelector('.command-prefixes__values')?.textContent).toContain('*')
    expect(previewDoc.querySelector('.command-guide')).toBeNull()
    const weatherUsages = Array.from(previewDoc.querySelectorAll('.command-usage'))
      .filter((usage) => usage.textContent?.includes('weather'))
    expect(weatherUsages.length).toBeGreaterThan(0)
    for (const usage of weatherUsages) {
      expect(usage.querySelectorAll('code')).toHaveLength(1)
      expect(usage.querySelector('.command-prefix-cue')?.textContent).toBe('前缀')
      expect(usage.querySelector('.command-usage__prefix-group')).toBeNull()
      expect(usage.querySelectorAll('.command-usage__name')).toHaveLength(1)
      expect(usage.querySelector('.command-argument--optional')?.textContent).toBe('[空气质量]')
      expect(usage.querySelector('.command-argument--required')?.textContent).toBe('<城市>')
    }
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

  it('renders pattern command bodies without repeating effective prefixes', async () => {
    const configStore = useConfigStore()
    const pluginsStore = usePluginsStore()
    configStore.document = {
      ...createConfig(),
      command: { prefixes: ['/', '*'] },
    } as ConfigDocument
    pluginsStore.items = [
      createPlugin({
        id: 'game-guide',
        name: '游戏攻略',
        commands: [
          {
            name: '已适配角色列表',
            description: '查看全部已适配角色',
            usage: '*角色列表',
            permission: 'everyone',
            command_source: 'pattern',
            declaration_id: 'character-list',
          },
          {
            name: '角色攻略',
            description: '按角色名或别名查询攻略图',
            usage: '*<角色名>攻略',
            permission: 'everyone',
            command_source: 'pattern',
            declaration_id: 'character-guide',
          },
          {
            name: '每日运势',
            description: '查看每日运势',
            usage: '每日运势 [日期]',
            permission: 'everyone',
            command_source: 'dynamic',
          },
        ],
        help: {
          summary: '星穹铁道攻略菜单',
          groups: [],
        },
      }),
    ]

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchList').mockResolvedValue(undefined)
    const wrapper = mount(MenuCenterView, {
      global: {
        plugins: [Antd],
      },
    })
    await flushPromises()

    const payload = pluginPreviewPayload(wrapper)
    expect(payload.groups).toHaveLength(1)
    expect(payload.groups[0].title).toBe('命令')
    expect(payload.groups[0].items[0]).toMatchObject({
      name: '已适配角色列表',
      command_source: 'pattern',
      command_prefixes: ['/', '*'],
      usage: '角色列表',
      usage_parts: [{ kind: 'literal', text: '角色列表' }],
    })
    expect(payload.groups[0].items[1]).toMatchObject({
      name: '角色攻略',
      command_source: 'pattern',
      command_prefixes: ['/', '*'],
      usage: '<角色名>攻略',
      usage_parts: [
        { kind: 'required', text: '角色名' },
        { kind: 'literal', text: '攻略' },
      ],
    })
    expect(payload.groups[0].items[2]).toMatchObject({
      name: '每日运势',
      command_source: 'dynamic',
      command_prefixes: ['/', '*'],
      usage_args: '[日期]',
      usage_parts: [{ kind: 'optional', text: '日期' }],
    })
    expect(payload.groups[0].items[2]).not.toHaveProperty('usage')

    const previewDoc = new DOMParser().parseFromString(pluginPreviewFrame(wrapper).attributes('srcdoc') ?? '', 'text/html')
    const cells = Array.from(previewDoc.querySelectorAll('.cell'))
    const listCell = cells.find((cell) => cell.querySelector('.meta')?.textContent === '已适配角色列表')
    const guideCell = cells.find((cell) => cell.querySelector('.meta')?.textContent === '角色攻略')
    const dynamicCell = cells.find((cell) => cell.querySelector('.meta')?.textContent === '每日运势')
    expect(previewDoc.querySelector('.command-prefixes__values')?.textContent?.trim()).toBe('/*')
    expect(previewDoc.querySelector('.command-guide')).toBeNull()
    expect(listCell?.querySelector('.command-prefix-cue')?.textContent).toBe('前缀')
    expect(guideCell?.querySelector('.command-prefix-cue')?.textContent).toBe('前缀')
    expect(dynamicCell?.querySelector('.command-prefix-cue')?.textContent).toBe('前缀')
    expect(listCell?.querySelector('.command-usage__prefix-group')).toBeNull()
    expect(listCell?.querySelector('.command-usage__name')?.textContent).toBe('角色列表')
    expect(guideCell?.querySelector('.command-argument--required')?.textContent).toBe('<角色名>')
    expect(guideCell?.querySelector('.command-usage__name')?.textContent).toBe('攻略')
    expect(dynamicCell?.querySelector('.command-usage__prefix-group')).toBeNull()
    expect(dynamicCell?.querySelector('.command-usage__name')?.textContent).toBe('每日运势')
    expect(dynamicCell?.querySelector('.command-argument--optional')?.textContent).toBe('[日期]')
    expect(previewDoc.body.textContent).not.toContain('*已适配角色列表')
  })

  it('strips multi-character configured prefixes and full-width fallback prefixes from pattern usages', async () => {
    const configStore = useConfigStore()
    const pluginsStore = usePluginsStore()
    configStore.document = {
      ...createConfig(),
      command: { prefixes: ['::', '/'] },
    } as ConfigDocument
    pluginsStore.items = [
      createPlugin({
        id: 'game-guide',
        name: '游戏攻略',
        commands: [
          {
            name: '多字符前缀示例',
            description: '测试多字符前缀',
            usage: '::角色列表',
            permission: 'everyone',
            command_source: 'pattern',
          },
          {
            name: '全角前缀示例',
            description: '测试全角前缀',
            usage: '＊<角色名>攻略',
            permission: 'everyone',
            command_source: 'pattern',
          },
        ],
        help: {
          summary: '星穹铁道攻略菜单',
          groups: [],
        },
      }),
    ]

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(pluginsStore, 'fetchList').mockResolvedValue(undefined)
    const wrapper = mount(MenuCenterView, {
      global: {
        plugins: [Antd],
      },
    })
    await flushPromises()

    const items = pluginPreviewPayload(wrapper).groups[0].items
    expect(items[0]).toMatchObject({
      name: '多字符前缀示例',
      command_prefixes: ['::', '/'],
      usage: '角色列表',
      usage_parts: [{ kind: 'literal', text: '角色列表' }],
    })
    expect(items[1]).toMatchObject({
      name: '全角前缀示例',
      command_prefixes: ['::', '/'],
      usage: '<角色名>攻略',
      usage_parts: [
        { kind: 'required', text: '角色名' },
        { kind: 'literal', text: '攻略' },
      ],
    })

    const previewDoc = new DOMParser().parseFromString(pluginPreviewFrame(wrapper).attributes('srcdoc') ?? '', 'text/html')
    const usages = Array.from(previewDoc.querySelectorAll('.command-usage'))
    expect(usages).toHaveLength(2)
    for (const usage of usages) {
      expect(usage.querySelector('.command-usage__prefix-group')).toBeNull()
    }
    expect(previewDoc.querySelector('.command-prefixes__values')?.textContent?.trim()).toBe('::/')
    expect(usages[0].querySelector('.command-usage__name')?.textContent).toBe('角色列表')
    expect(usages[1].querySelector('.command-argument--required')?.textContent).toBe('<角色名>')
    expect(usages[1].querySelector('.command-usage__name')?.textContent).toBe('攻略')
    expect(previewDoc.body.textContent).not.toContain('多字符前缀示例::')
    expect(previewDoc.body.textContent).not.toContain('全角前缀示例::')
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
    expect(preview).toContain('LXGW WenKai Medium')
    expect(preview).not.toMatch(/\.ttf\b/i)
  })

  it('keeps native preview classes aligned with the canonical help menu template and stylesheet', () => {
    const wrapper = mount(NativeTemplatePreviewFrame, {
      props: {
        templateId: 'help.menu',
        data: {
          title: '插件菜单',
          subtitle: '当前可用插件',
          command_prefixes: ['/', '*'],
          trigger_examples: ['/help 天气', '*天气帮助'],
          user: {
            id: '10001',
            nickname: '星野',
            title: '指令调度员',
          },
          group: {
            name: '测试群组',
          },
          permission: {
            level: 'admin',
          },
          groups: [
            {
              title: '命令',
              items: [
                {
                  name: 'weather',
                  description: '查询天气',
                  command_prefixes: ['/', '*'],
                  command_source: 'manifest',
                  usage_args: '[空气质量] 城市',
                  usage_parts: [
                    { kind: 'optional', text: '空气质量' },
                    { kind: 'required', text: '城市' },
                  ],
                  permission: 'everyone',
                  permission_label: '所有人',
                },
                {
                  name: 'legacy',
                  description: '兼容旧输入',
                  command_prefixes: ['/'],
                  command_source: 'manifest',
                  usage_args: '参数',
                  permission: 'group_admin',
                  permission_label: '群管理员',
                },
              ],
            },
          ],
          render_footer: nativeMenuPreviewFooter,
        },
      },
    })

    const srcdoc = wrapper.get('[data-testid="native-template-preview-frame"]').attributes('srcdoc') ?? ''
    const previewClasses = htmlClassNames(srcdoc)
    const templateClasses = htmlClassNames(helpMenuTemplate.replace(/\{\{[\s\S]*?\}\}/g, ' '))
    const stylesheetClasses = cssClassNames(helpMenuStyles)

    for (const className of previewClasses) {
      if (className.startsWith('theme-')) {
        continue
      }
      const dynamicModifierPrefix = className.includes('--') ? `${className.slice(0, className.lastIndexOf('--') + 2)}{{` : ''
      const declaredByTemplate = templateClasses.has(className)
        || Boolean(dynamicModifierPrefix && helpMenuTemplate.includes(dynamicModifierPrefix))
      const baseClassName = className.includes('--') ? className.slice(0, className.lastIndexOf('--')) : ''
      const styledByStylesheet = stylesheetClasses.has(className)
        || Boolean(baseClassName && stylesheetClasses.has(baseClassName))
      expect(declaredByTemplate, `preview class .${className} is missing from templates/help.menu/template.html`).toBe(true)
      expect(styledByStylesheet, `preview class .${className} is missing from templates/help.menu/styles.css`).toBe(true)
    }

    for (const className of templateClasses) {
      if (
        className.startsWith('theme-')
        || className.startsWith('permission-badge--')
        || className.startsWith('command-permission--')
      ) {
        continue
      }
      expect(previewClasses.has(className), `canonical class .${className} is missing from the native preview`).toBe(true)
    }
  })

  it('keeps command usage visibility aligned with the canonical template', () => {
    const wrapper = mount(NativeTemplatePreviewFrame, {
      props: {
        templateId: 'help.menu',
        data: {
          title: '插件菜单',
          groups: [{
            title: '命令',
            items: [{
              name: 'weather',
              description: '查询天气',
              command_source: 'manifest',
              usage_parts: [{ kind: 'required', text: '城市' }],
            }],
          }],
        },
      },
    })

    expect(wrapper.get('[data-testid="native-template-preview-frame"]').attributes('srcdoc'))
      .not.toContain('class="command-usage"')
  })
})

function htmlClassNames(source: string) {
  const document = new DOMParser().parseFromString(source, 'text/html')
  return new Set(Array.from(document.querySelectorAll('[class]')).flatMap((element) => Array.from(element.classList)))
}

function cssClassNames(source: string) {
  const selectors = source
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/url\([^)]*\)/gi, '')
    .replace(/"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'/g, '')
  return new Set(Array.from(selectors.matchAll(/\.([_a-zA-Z]+[\w-]*)/g), (match) => match[1]))
}

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
