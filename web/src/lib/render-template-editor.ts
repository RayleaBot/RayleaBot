import type { RenderTemplateLocalIssue, RenderTemplateSchemaNode } from '@/types/api'

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function normalizeJsonError(error: unknown) {
  if (error instanceof Error && error.message) {
    return `JSON 解析失败：${error.message}`
  }

  return 'JSON 解析失败，请检查格式。'
}

export function parseRenderTemplatePreviewData(raw: string) {
  const text = raw.trim()
  if (!text) {
    return {
      data: {},
      issue: null,
    }
  }

  try {
    const parsed = JSON.parse(text)
    if (!isPlainObject(parsed)) {
      return {
        data: null,
        issue: {
          field: 'preview_data',
          message: '预览输入需要是 JSON 对象。',
        } satisfies RenderTemplateLocalIssue,
      }
    }

    return {
      data: parsed,
      issue: null,
    }
  } catch (error) {
    return {
      data: null,
      issue: {
        field: 'preview_data',
        message: normalizeJsonError(error),
      } satisfies RenderTemplateLocalIssue,
    }
  }
}

function normalizeSchemaType(schema: Record<string, unknown>) {
  const rawType = schema.type
  if (Array.isArray(rawType)) {
    const values = rawType.filter((value): value is string => typeof value === 'string')
    if (values.length > 0) {
      return values.join(' | ')
    }
  }

  if (typeof rawType === 'string' && rawType) {
    return rawType
  }

  if (isPlainObject(schema.properties)) {
    return 'object'
  }

  if (schema.items) {
    return 'array'
  }

  return 'unknown'
}

function collectSchemaNodes(
  nodes: RenderTemplateSchemaNode[],
  schema: Record<string, unknown>,
  options: {
    depth: number
    label: string
    path: string
    required: boolean
  },
) {
  nodes.push({
    key: options.path || '$root',
    path: options.path || '$root',
    label: options.label,
    type: normalizeSchemaType(schema),
    required: options.required,
    description: typeof schema.description === 'string' ? schema.description : '',
    depth: options.depth,
  })

  if (isPlainObject(schema.properties)) {
    const requiredSet = new Set(
      Array.isArray(schema.required)
        ? schema.required.filter((value): value is string => typeof value === 'string')
        : [],
    )

    for (const [key, value] of Object.entries(schema.properties).sort(([left], [right]) => left.localeCompare(right))) {
      if (!isPlainObject(value)) {
        continue
      }

      collectSchemaNodes(nodes, value, {
        depth: options.depth + 1,
        label: key,
        path: options.path ? `${options.path}.${key}` : key,
        required: requiredSet.has(key),
      })
    }
  }

  if (isPlainObject(schema.items)) {
    collectSchemaNodes(nodes, schema.items, {
      depth: options.depth + 1,
      label: '[items]',
      path: options.path ? `${options.path}[]` : '[]',
      required: true,
    })
  }
}

export function buildRenderTemplateSchemaNodes(schema: Record<string, unknown> | null): RenderTemplateSchemaNode[] {
  if (!schema) {
    return []
  }

  const nodes: RenderTemplateSchemaNode[] = []
  collectSchemaNodes(nodes, schema, {
    depth: 0,
    label: 'root',
    path: '',
    required: true,
  })
  return nodes
}
