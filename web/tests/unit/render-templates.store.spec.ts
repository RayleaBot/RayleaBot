import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useRenderTemplatesStore } from '@/stores/render-templates'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function templateDetail() {
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

function templateSource(revisionId = 'rev_help_menu_0004') {
  return {
    template_id: 'help.menu',
    revision_id: revisionId,
    source: {
      manifest_json: {
        id: 'help.menu',
        version: '1',
        entry_html: 'template.html',
        stylesheet: 'styles.css',
        input_schema: 'input.schema.json',
        width: 960,
        height: 640,
      },
      html: '<section class="menu-card"><h1>{{ .title }}</h1></section>',
      stylesheet: '.menu-card { display: grid; gap: 12px; }',
      input_schema_json: {
        type: 'object',
        properties: {
          title: { type: 'string' },
        },
      },
    },
  } as const
}

function templateVersions() {
  return {
    items: [
      {
        revision_id: 'rev_help_menu_0004',
        template_version: '1',
        saved_at: '2026-04-18T10:30:00Z',
        kind: 'save',
        message: '调整帮助菜单排版',
      },
    ],
  } as const
}

describe('render templates store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps the session draft when workspace data refreshes without reset', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn()
        .mockResolvedValueOnce(jsonResponse({ template: templateDetail() }))
        .mockResolvedValueOnce(jsonResponse(templateSource()))
        .mockResolvedValueOnce(jsonResponse(templateVersions())),
    )

    const store = useRenderTemplatesStore()
    store.draftById = {
      'help.menu': {
        manifest_json: '{\n  "id": "help.menu"\n}',
        html: '<section class="menu-card"><h1>本地草稿</h1></section>',
        stylesheet: '.menu-card { gap: 20px; }',
        input_schema_json: '{\n  "type": "object"\n}',
      },
    }

    await store.fetchTemplateWorkspace('help.menu')

    expect(store.draftById['help.menu']?.html).toContain('本地草稿')
    expect(store.baseDraftById['help.menu']?.html).toContain('{{ .title }}')
    expect(store.versionsById['help.menu']).toHaveLength(1)
  })

  it('marks a template conflict and preserves the local draft after stale save', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse({
        error: {
          code: 'platform.template_revision_conflict',
          message: '模板版本已变化',
          message_key: 'errors.platform.template_revision_conflict',
          request_id: 'req_template_conflict_0001',
        },
      }, 409)),
    )

    const store = useRenderTemplatesStore()
    store.detailById = {
      'help.menu': templateDetail(),
    }
    store.sourceMetaById = {
      'help.menu': {
        template_id: 'help.menu',
        revision_id: 'rev_help_menu_0004',
      },
    }
    store.baseDraftById = {
      'help.menu': {
        manifest_json: '{\n  "id": "help.menu"\n}',
        html: '<section class="menu-card"><h1>{{ .title }}</h1></section>',
        stylesheet: '.menu-card { gap: 12px; }',
        input_schema_json: '{\n  "type": "object"\n}',
      },
    }
    store.draftById = {
      'help.menu': {
        manifest_json: '{\n  "id": "help.menu"\n}',
        html: '<section class="menu-card"><h1>冲突中的本地草稿</h1></section>',
        stylesheet: '.menu-card { gap: 20px; }',
        input_schema_json: '{\n  "type": "object"\n}',
      },
    }

    await expect(store.saveTemplate('help.menu', {
      base_revision_id: 'rev_help_menu_0004',
      message: '覆盖旧版本',
      source: {
        manifest_json: { id: 'help.menu' },
        html: '<section class="menu-card"><h1>冲突中的本地草稿</h1></section>',
        stylesheet: '.menu-card { gap: 20px; }',
        input_schema_json: { type: 'object' },
      },
    })).rejects.toThrow()

    expect(store.conflictById['help.menu']).toBe(true)
    expect(store.draftById['help.menu']?.html).toContain('冲突中的本地草稿')
  })
})
