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

function firstSchemaExample(schema: Record<string, unknown>) {
  if (isPlainObject(schema.default)) {
    return schema.default
  }

  if (Array.isArray(schema.examples)) {
    const example = schema.examples.find((value) => isPlainObject(value))
    if (isPlainObject(example)) {
      return example
    }
  }

  return null
}

function firstScalarExample(schema: Record<string, unknown>) {
  if ('default' in schema && !isPlainObject(schema.default) && !Array.isArray(schema.default)) {
    return schema.default
  }

  if (Array.isArray(schema.examples) && schema.examples.length > 0) {
    const example = schema.examples.find((value) => !isPlainObject(value) && !Array.isArray(value))
    if (example !== undefined) {
      return example
    }
  }

  if (Array.isArray(schema.enum) && schema.enum.length > 0) {
    return schema.enum[0]
  }

  return undefined
}

function schemaTypes(schema: Record<string, unknown>) {
  const rawType = schema.type
  if (Array.isArray(rawType)) {
    return rawType.filter((value): value is string => typeof value === 'string')
  }
  return typeof rawType === 'string' ? [rawType] : []
}

function sampleStringForKey(key: string) {
  const normalized = key.toLowerCase()
  if (normalized.includes('title')) {
    return '示例标题'
  }
  if (normalized.includes('subtitle') || normalized.includes('summary') || normalized.includes('description')) {
    return '示例说明'
  }
  if (normalized.includes('avatar') || normalized.includes('url')) {
    return 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=100'
  }
  if (normalized.includes('nickname') || normalized.includes('name')) {
    return '示例成员'
  }
  if (normalized.includes('label')) {
    return '数值'
  }
  if (normalized.includes('status')) {
    return 'ready'
  }
  return '示例文本'
}

function buildSchemaSampleValue(schema: Record<string, unknown>, key: string): unknown {
  const objectExample = firstSchemaExample(schema)
  if (objectExample) {
    return objectExample
  }

  const scalarExample = firstScalarExample(schema)
  if (scalarExample !== undefined) {
    return scalarExample
  }

  const types = schemaTypes(schema)
  if (types.includes('object') || isPlainObject(schema.properties)) {
    const properties = isPlainObject(schema.properties) ? schema.properties : {}
    const requiredSet = new Set(
      Array.isArray(schema.required)
        ? schema.required.filter((value): value is string => typeof value === 'string')
        : [],
    )
    const keys = Object.keys(properties).filter((propertyKey) => requiredSet.has(propertyKey))
    return Object.fromEntries(keys.map((propertyKey) => {
      const propertySchema = properties[propertyKey]
      return [
        propertyKey,
        isPlainObject(propertySchema) ? buildSchemaSampleValue(propertySchema, propertyKey) : null,
      ]
    }))
  }

  if (types.includes('array') || schema.items) {
    return isPlainObject(schema.items) ? [buildSchemaSampleValue(schema.items, key)] : []
  }

  if (types.includes('number') || types.includes('integer')) {
    return 1
  }
  if (types.includes('boolean')) {
    return true
  }
  return sampleStringForKey(key)
}

export function buildRenderTemplatePreviewSample(schema: Record<string, unknown> | null) {
  if (!schema) {
    return {}
  }

  const sample = buildSchemaSampleValue(schema, '')
  return isPlainObject(sample) ? sample : {}
}
