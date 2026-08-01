#!/usr/bin/env node
// Sync runtime schema copies from contracts/ into the server module so Go can
// embed them via go:embed (embed cannot reference files outside the module).
// Default mode copies with LF normalization; --verify checks the copies match.
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const targetDir = 'server/internal/config/contracts'

const schemas = ['config.user.schema.json', 'plugin-info.schema.json', 'plugin-artifact.schema.json']
const pluginUIBridgeSchema = 'plugin-management-ui-bridge.schema.json'
const pluginUITypesTarget = 'sdk/vue/src/contract.generated.ts'

const verifyMode = process.argv.includes('--verify')
let failed = false

for (const name of schemas) {
  const sourcePath = path.join(repoRoot, 'contracts', name)
  const targetPath = path.join(repoRoot, targetDir, name)
  const normalized = normalizeSchemaBytes(await fs.readFile(sourcePath))

  if (verifyMode) {
    let current = null
    try {
      current = normalizeSchemaBytes(await fs.readFile(targetPath))
    } catch {
      console.error(`missing embedded schema copy: ${targetDir}/${name}`)
      failed = true
      continue
    }
    if (!normalized.equals(current)) {
      console.error(`embedded schema copy is out of sync with contracts/${name}; run node scripts/generate-runtime-schemas.mjs`)
      failed = true
    }
    continue
  }

  await fs.mkdir(path.dirname(targetPath), { recursive: true })
  await fs.writeFile(targetPath, normalized)
}

const bridgeSchema = JSON.parse(await fs.readFile(path.join(repoRoot, 'contracts', pluginUIBridgeSchema), 'utf8'))
const generatedPluginUITypes = Buffer.from(generatePluginUITypes(bridgeSchema), 'utf8')
const pluginUITypesPath = path.join(repoRoot, pluginUITypesTarget)
if (verifyMode) {
  let current = null
  try {
    current = normalizeSchemaBytes(await fs.readFile(pluginUITypesPath))
  } catch {
    console.error(`missing generated plugin UI types: ${pluginUITypesTarget}`)
    failed = true
  }
  if (current && !generatedPluginUITypes.equals(current)) {
    console.error(`generated plugin UI types are out of sync with contracts/${pluginUIBridgeSchema}; run node scripts/generate-runtime-schemas.mjs`)
    failed = true
  }
} else {
  await fs.mkdir(path.dirname(pluginUITypesPath), { recursive: true })
  await fs.writeFile(pluginUITypesPath, generatedPluginUITypes)
}

if (failed) {
  process.exit(1)
}

function normalizeSchemaBytes(buffer) {
  return Buffer.from(buffer.toString('utf8').replace(/\r\n?/g, '\n'), 'utf8')
}

function generatePluginUITypes(schema) {
  const definitions = schema.$defs ?? {}
  const sourceValues = schema.properties?.source?.enum
  const typeValues = schema.properties?.type?.enum
  const version = schema.properties?.version?.const
  if (!Array.isArray(sourceValues) || !Array.isArray(typeValues) || typeof version !== 'string') {
    throw new Error(`${pluginUIBridgeSchema} is missing the version, source, or type contract`)
  }

  const renderDefinition = (name) => {
    const definition = definitions[name]
    if (!definition) {
      throw new Error(`${pluginUIBridgeSchema} is missing $defs.${name}`)
    }
    return renderType(definition, schema, 0)
  }

  return [
    '// Code generated from contracts/plugin-management-ui-bridge.schema.json; DO NOT EDIT.',
    '',
    `export type BridgeSource = ${literalUnion(sourceValues)}`,
    '',
    `export type BridgeType = ${literalUnion(typeValues, '  | ')}`,
    '',
    'export interface BridgeMessage<T = unknown> {',
    `  version: ${JSON.stringify(version)}`,
    '  source: BridgeSource',
    '  type: BridgeType',
    '  nonce?: string',
    '  request_id?: string',
    '  payload?: T',
    '}',
    '',
    `export type PluginDescriptor = ${renderDefinition('plugin_summary')}`,
    '',
    `export type PluginPageDescriptor = ${renderDefinition('page_summary')}`,
    '',
    `export type HostInitPayload = ${renderDefinition('host_init_payload')}`,
    '',
    `export type SettingsChangedPayload = ${renderType(definitions.host_config_message.properties.payload, schema, 0)}`,
    '',
    `export type SecretsStatusPayload = ${renderType(definitions.host_secret_status_message.properties.payload, schema, 0)}`,
    '',
    `export type BridgeErrorPayload = ${renderDefinition('error_payload')}`,
    '',
  ].join('\n')
}

function literalUnion(values, separator = ' | ') {
  return values.map((value) => JSON.stringify(value)).join(`\n${separator}`)
}

function renderType(input, root, level) {
  if (input === false) return 'never'
  if (!input || input === true) return 'unknown'
  if (input.$ref) {
    const prefix = '#/$defs/'
    if (!input.$ref.startsWith(prefix)) {
      throw new Error(`unsupported schema reference ${input.$ref}`)
    }
    return renderType(root.$defs[input.$ref.slice(prefix.length)], root, level)
  }
  if (Object.hasOwn(input, 'const')) return JSON.stringify(input.const)
  if (Array.isArray(input.enum)) return input.enum.map((value) => JSON.stringify(value)).join(' | ')
  if (Array.isArray(input.oneOf)) return input.oneOf.map((item) => renderType(item, root, level)).join(' | ')
  if (Array.isArray(input.anyOf)) return input.anyOf.map((item) => renderType(item, root, level)).join(' | ')
  if (input.type === 'string') return 'string'
  if (input.type === 'boolean') return 'boolean'
  if (input.type === 'number' || input.type === 'integer') return 'number'
  if (input.type === 'array') return `Array<${renderType(input.items, root, level)}>`
  if (input.type !== 'object') return 'unknown'

  const properties = input.properties ?? {}
  const names = Object.keys(properties)
  if (names.length === 0) {
    if (input.additionalProperties === false) return 'Record<string, never>'
    if (input.additionalProperties && input.additionalProperties !== true) {
      return `Record<string, ${renderType(input.additionalProperties, root, level)}>`
    }
    return 'Record<string, unknown>'
  }

  const required = new Set(input.required ?? [])
  const indentation = '  '.repeat(level)
  const childIndentation = '  '.repeat(level + 1)
  const lines = names.map((name) => {
    const key = /^[A-Za-z_$][A-Za-z0-9_$]*$/.test(name) ? name : JSON.stringify(name)
    const optional = required.has(name) ? '' : '?'
    const value = renderType(properties[name], root, level + 1)
    return `${childIndentation}${key}${optional}: ${indentMultiline(value, childIndentation)}`
  })
  const objectType = `{\n${lines.join('\n')}\n${indentation}}`
  if (input.additionalProperties === true || (input.additionalProperties && input.additionalProperties !== false)) {
    const additional = input.additionalProperties === true ? 'unknown' : renderType(input.additionalProperties, root, level)
    return `(${objectType} & Record<string, ${additional}>)`
  }
  return objectType
}

function indentMultiline(value, indentation) {
  return value.replaceAll('\n', `\n${indentation}`)
}
