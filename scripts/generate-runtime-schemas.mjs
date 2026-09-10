#!/usr/bin/env node
// Sync runtime schema copies from contracts/ into the server module so Go can
// embed them via go:embed (embed cannot reference files outside the module).
// Default mode copies with LF normalization; --verify checks the copies match.
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const targetDir = 'server/internal/config/contracts'

const schemas = [
  'backup-manifest.schema.json',
  'config.user.schema.json',
  'plugin-info.schema.json',
  'plugin-artifact.schema.json',
  'plugin-store-catalog.schema.json',
]
const pluginUIBridgeSchema = 'plugin-management-ui-bridge.schema.json'
const pluginUITypesTargets = [
  'sdk/vue/src/contract.generated.ts',
  'web/src/types/plugin-management-ui.generated.ts',
]

const verifyMode = process.argv.includes('--verify')
let failed = false

const schemaCopies = [
  ...schemas.map(name => ({ name, target: `${targetDir}/${name}` })),
  { name: 'deps-manifest.schema.json', target: 'server/internal/platform/deps/contracts/deps-manifest.schema.json' },
  { name: 'deps-manifest.schema.json', target: 'launcher/internal/desktop/contracts/deps-manifest.schema.json' },
]

// These directories contain only schema copies owned by this generator. TS
// directories are shared, so recognize ownership by the generated header.
const expectedPaths = new Set([...schemaCopies.map(item => item.target), ...pluginUITypesTargets])
for (const directory of new Set([...expectedPaths].map(target => path.posix.dirname(target)))) {
  const entries = await fs.readdir(path.join(repoRoot, directory), { withFileTypes: true }).catch(error => {
    if (error.code === 'ENOENT') return []
    throw error
  })
  for (const entry of entries) {
    if (!entry.isFile()) continue
    const target = `${directory}/${entry.name}`
    if (expectedPaths.has(target)) continue
    const fullPath = path.join(repoRoot, target)
    const schemaCopy = schemaCopies.some(item => path.posix.dirname(item.target) === directory) && entry.name.endsWith('.schema.json')
    const bridgeCopy = entry.name.endsWith('.ts') && (await fs.readFile(fullPath, 'utf8')).startsWith('// Code generated from contracts/plugin-management-ui-bridge.schema.json;')
    if (!schemaCopy && !bridgeCopy) continue
    if (verifyMode) { console.error(`stale generated runtime schema output: ${target}`); failed = true }
    else await fs.unlink(fullPath)
  }
}

for (const { name, target } of schemaCopies) {
  const sourcePath = path.join(repoRoot, 'contracts', name)
  const targetPath = path.join(repoRoot, target)
  const normalized = normalizeSchemaBytes(await fs.readFile(sourcePath))

  if (verifyMode) {
    let current = null
    try {
      current = normalizeSchemaBytes(await fs.readFile(targetPath))
    } catch {
      console.error(`missing embedded schema copy: ${target}`)
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
  await writeIfChanged(targetPath, normalized)
}

const bridgeSchema = JSON.parse(await fs.readFile(path.join(repoRoot, 'contracts', pluginUIBridgeSchema), 'utf8'))
const generatedPluginUITypes = Buffer.from(generatePluginUITypes(bridgeSchema), 'utf8')
for (const pluginUITypesTarget of pluginUITypesTargets) {
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
    await writeIfChanged(pluginUITypesPath, generatedPluginUITypes)
  }
}

if (failed) {
  process.exit(1)
}

function normalizeSchemaBytes(buffer) {
  return Buffer.from(buffer.toString('utf8').replace(/\r\n?/g, '\n'), 'utf8')
}

async function writeIfChanged(target, data) {
  const current = await fs.readFile(target).catch(error => {
    if (error.code === 'ENOENT') return null
    throw error
  })
  if (!current?.equals(data)) await fs.writeFile(target, data)
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
    `export const PLUGIN_UI_BRIDGE_VERSION = ${JSON.stringify(version)} as const`,
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
