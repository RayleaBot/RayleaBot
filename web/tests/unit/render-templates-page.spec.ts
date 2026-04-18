import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import RenderTemplatesView from '@/views/system/RenderTemplatesView.vue'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import { useSystemStore } from '@/stores/system'
import { useTasksStore } from '@/stores/tasks'

function createTemplateDetail() {
  return {
    id: 'help.menu',
    version: '1',
    width: 960,
    height: 640,
    has_input_schema: true,
    current_revision_id: 'rev_help_menu_0004',
    updated_at: '2026-04-18T10:30:00Z',
    files: {
      manifest: 'template.json',
      html: 'template.html',
      stylesheet: 'styles.css',
      input_schema: 'input.schema.json',
    },
    current_revision: {
      revision_id: 'rev_help_menu_0004',
      template_version: '1',
      saved_at: '2026-04-18T10:30:00Z',
      kind: 'save',
      message: '调整帮助菜单排版',
    },
    last_validation: {
      valid: true,
      checked_at: '2026-04-18T10:31:00Z',
      issue_count: 0,
    },
  } as const
}

function createDraft() {
  return {
    manifest_json: JSON.stringify({
      id: 'help.menu',
      version: '1',
      entry_html: 'template.html',
      stylesheet: 'styles.css',
      input_schema: 'input.schema.json',
      width: 960,
      height: 640,
    }, null, 2),
    html: '<section class="menu-card"><h1>{{ .title }}</h1></section>',
    stylesheet: '.menu-card { display: grid; gap: 12px; }',
    input_schema_json: JSON.stringify({
      type: 'object',
      properties: {
        title: {
          type: 'string',
          description: '主标题',
        },
      },
      required: ['title'],
    }, null, 2),
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

describe('RenderTemplatesView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(new Blob(['fixture']), {
      status: 200,
      headers: {
        'Content-Type': 'image/png',
      },
    })))
  })

  it('renders the template workspace and can submit a preview task from the page', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()
    const systemStore = useSystemStore()
    const tasksStore = useTasksStore()

    renderTemplatesStore.items = [
      {
        id: 'help.menu',
        version: '1',
        width: 960,
        height: 640,
        has_input_schema: true,
        current_revision_id: 'rev_help_menu_0004',
        updated_at: '2026-04-18T10:30:00Z',
      },
    ]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }
    renderTemplatesStore.draftById = {
      'help.menu': createDraft(),
    }
    renderTemplatesStore.baseDraftById = {
      'help.menu': createDraft(),
    }
    renderTemplatesStore.versionsById = {
      'help.menu': [
        {
          revision_id: 'rev_help_menu_0004',
          template_version: '1',
          saved_at: '2026-04-18T10:30:00Z',
          kind: 'save',
          message: '调整帮助菜单排版',
        },
      ],
    }
    renderTemplatesStore.validationById = {
      'help.menu': {
        valid: true,
        issues: [],
        normalized_manifest: {
          id: 'help.menu',
          version: '1',
        },
      },
    }
    renderTemplatesStore.sourceMetaById = {
      'help.menu': {
        template_id: 'help.menu',
        revision_id: 'rev_help_menu_0004',
      },
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue({
      detail: renderTemplatesStore.detailById['help.menu'],
      source: {
        manifest_json: { id: 'help.menu', version: '1' },
        html: createDraft().html,
        stylesheet: createDraft().stylesheet,
        input_schema_json: { type: 'object' },
      },
      versions: renderTemplatesStore.versionsById['help.menu'],
    })

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

    const router = createRouterForPage()
    await router.push('/render/templates/help.menu')
    await router.isReady()

    const wrapper = mount(RenderTemplatesView, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('模板编辑')
    expect(wrapper.text()).toContain('help.menu')
    expect(wrapper.text()).toContain('输入结构')
    expect(wrapper.text()).toContain('主标题')

    await wrapper.get('[data-testid="render-template-preview-button"]').trigger('click')
    await flushPromises()

    expect(systemStore.previewRender).toHaveBeenCalledTimes(1)
    expect(tasksStore.fetchTask).toHaveBeenCalledWith('task_render_preview_0001', { makeCurrent: false })
    expect(wrapper.get('[data-testid="render-template-preview-result"]').text()).toContain('render_preview_0001.png')
    expect(wrapper.text()).toContain('打开任务详情')
  })

  it('shows a local parse error and blocks validation when manifest json is invalid', async () => {
    const renderTemplatesStore = useRenderTemplatesStore()

    renderTemplatesStore.items = [
      {
        id: 'help.menu',
        version: '1',
        width: 960,
        height: 640,
        has_input_schema: true,
        current_revision_id: 'rev_help_menu_0004',
        updated_at: '2026-04-18T10:30:00Z',
      },
    ]
    renderTemplatesStore.detailById = {
      'help.menu': createTemplateDetail(),
    }
    renderTemplatesStore.draftById = {
      'help.menu': createDraft(),
    }
    renderTemplatesStore.baseDraftById = {
      'help.menu': createDraft(),
    }
    renderTemplatesStore.versionsById = {
      'help.menu': [],
    }
    renderTemplatesStore.sourceMetaById = {
      'help.menu': {
        template_id: 'help.menu',
        revision_id: 'rev_help_menu_0004',
      },
    }

    vi.spyOn(renderTemplatesStore, 'fetchTemplates').mockResolvedValue({ items: renderTemplatesStore.items })
    vi.spyOn(renderTemplatesStore, 'fetchTemplateWorkspace').mockResolvedValue({
      detail: renderTemplatesStore.detailById['help.menu'],
      source: {
        manifest_json: { id: 'help.menu', version: '1' },
        html: createDraft().html,
        stylesheet: createDraft().stylesheet,
        input_schema_json: { type: 'object' },
      },
      versions: [],
    })
    const validateSpy = vi.spyOn(renderTemplatesStore, 'validateTemplate').mockResolvedValue({
      valid: true,
      issues: [],
      normalized_manifest: { id: 'help.menu' },
    })

    const router = createRouterForPage()
    await router.push('/render/templates/help.menu')
    await router.isReady()

    const wrapper = mount(RenderTemplatesView, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    await wrapper.find('textarea').setValue('{')
    const validateButton = wrapper.findAll('button').find((button) => button.text().includes('执行校验'))
    expect(validateButton).toBeDefined()
    await validateButton!.trigger('click')
    await flushPromises()

    expect(validateSpy).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('本地解析错误')
  })
})
