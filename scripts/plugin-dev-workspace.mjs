import fs from 'node:fs'
import fsp from 'node:fs/promises'
import path from 'node:path'

export const PLUGIN_DEV_OFF = 'off'
export const PLUGIN_DEV_SYNC = 'sync'
export const PLUGIN_DEV_WATCH = 'watch'

const validModes = new Set([PLUGIN_DEV_OFF, PLUGIN_DEV_SYNC, PLUGIN_DEV_WATCH])
const ignoredDirectoryNames = new Set(['.git', '.rayleabot', 'dist', 'node_modules'])

export function currentPluginPlatform(platform = process.platform, arch = process.arch) {
  switch (`${platform}/${arch}`) {
    case 'win32/x64': return 'windows-x64'
    case 'linux/x64': return 'linux-x64'
    case 'darwin/arm64': return 'macos-arm64'
    default: throw new Error(`Unsupported plugin development platform: ${platform}/${arch}`)
  }
}

export function resolvePluginDevMode(env, workspaceExists) {
  const raw = String(env.RAYLEA_PLUGIN_DEV ?? '').trim().toLowerCase()
  if (!raw) {
    return workspaceExists ? PLUGIN_DEV_SYNC : PLUGIN_DEV_OFF
  }
  if (!validModes.has(raw)) {
    throw new Error('RAYLEA_PLUGIN_DEV must be off, sync, or watch.')
  }
  return raw
}

export async function loadPluginWorkspace(workspacePath) {
  let raw
  try {
    raw = await fsp.readFile(workspacePath, 'utf8')
  } catch (error) {
    if (error?.code === 'ENOENT') {
      return { workspaceVersion: '1', plugins: [] }
    }
    throw error
  }
  const document = JSON.parse(raw)
  if (!document || typeof document !== 'object' || Array.isArray(document)
    || document.workspace_version !== '1' || !Array.isArray(document.plugins)) {
    throw new Error(`${workspacePath} does not satisfy plugin development workspace v1.`)
  }
  const allowedRootKeys = new Set(['workspace_version', 'plugins'])
  if (Object.keys(document).some((key) => !allowedRootKeys.has(key))) {
    throw new Error(`${workspacePath} contains an unsupported top-level field.`)
  }
  const workspaceDir = path.dirname(workspacePath)
  const seen = new Set()
  const plugins = document.plugins.map((entry, index) => {
    const allowedKeys = new Set(['id', 'path', 'enabled'])
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)
      || Object.keys(entry).some((key) => !allowedKeys.has(key))
      || typeof entry.id !== 'string' || !/^[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?$/.test(entry.id)
      || typeof entry.path !== 'string' || !entry.path.trim()
      || (entry.enabled !== undefined && typeof entry.enabled !== 'boolean')) {
      throw new Error(`${workspacePath} has an invalid plugins[${index}] entry.`)
    }
    if (seen.has(entry.id)) {
      throw new Error(`${workspacePath} declares duplicate plugin id ${entry.id}.`)
    }
    seen.add(entry.id)
    return {
      id: entry.id,
      path: path.resolve(workspaceDir, entry.path),
      enabled: entry.enabled !== false,
    }
  }).filter((entry) => entry.enabled)
  return { workspaceVersion: '1', plugins }
}

export async function collectWorkspaceSDKVersions(plugins) {
  const versions = new Set()
  for (const plugin of plugins) {
    const goMod = await fsp.readFile(path.join(plugin.path, 'go.mod'), 'utf8')
    const matches = goMod.matchAll(/github\.com\/RayleaBot\/RayleaBot\/sdk\/go\s+(v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)/g)
    for (const match of matches) {
      versions.add(match[1])
    }
  }
  return [...versions].sort()
}

export function renderDevelopmentGoWork({ sdkGoPath, sdkGoVersions = [], plugins, goVersion = '1.25.12' }) {
  const modulePaths = [sdkGoPath, ...plugins.map((plugin) => plugin.path)]
  const uniquePaths = [...new Set(modulePaths.map((modulePath) => path.resolve(modulePath)))]
  const uses = uniquePaths.map((modulePath) => `\t${quoteGoWorkPath(modulePath)}`).join('\n')
  const replacements = [...new Set(sdkGoVersions)]
    .map((version) => `replace github.com/RayleaBot/RayleaBot/sdk/go ${version} => ${quoteGoWorkPath(sdkGoPath)}`)
    .join('\n')
  return `go ${goVersion}\n\nuse (\n${uses}\n)\n${replacements ? `\n${replacements}\n` : ''}`
}

export async function mirrorVueSDK({ sdkVuePath, pluginPath }) {
  const uiPackage = path.join(pluginPath, 'ui', 'package.json')
  if (!fs.existsSync(uiPackage)) {
    return
  }
  const target = path.join(pluginPath, '.rayleabot', 'sdk', 'vue')
  await fsp.rm(target, { recursive: true, force: true })
  await fsp.mkdir(path.dirname(target), { recursive: true })
  await fsp.cp(sdkVuePath, target, {
    recursive: true,
    filter: (source) => !['node_modules', 'dist'].includes(path.basename(source)),
  })
}

export async function watchPluginWorkspace(plugins, onChange) {
  const watchers = []
  for (const plugin of plugins) {
    await watchDirectory(plugin.path, plugin, onChange, watchers)
  }
  return async () => {
    for (const watcher of watchers) {
      watcher.close()
    }
  }
}

async function watchDirectory(directory, plugin, onChange, watchers) {
  let entries
  try {
    entries = await fsp.readdir(directory, { withFileTypes: true })
  } catch (error) {
    if (error?.code === 'ENOENT') return
    throw error
  }
  const watcher = fs.watch(directory, (eventType, filename) => {
    if (!filename) return
    const sourcePath = path.join(directory, filename.toString())
    if (!isIgnoredPath(plugin.path, sourcePath)) {
      onChange(plugin, sourcePath)
    }
    if (eventType === 'rename') {
      void watchNewDirectory(sourcePath, plugin, onChange, watchers)
    }
  })
  watchers.push(watcher)
  await Promise.all(entries
    .filter((entry) => entry.isDirectory() && !ignoredDirectoryNames.has(entry.name))
    .map((entry) => watchDirectory(path.join(directory, entry.name), plugin, onChange, watchers)))
}

async function watchNewDirectory(directory, plugin, onChange, watchers) {
  try {
    const stat = await fsp.stat(directory)
    if (stat.isDirectory() && !ignoredDirectoryNames.has(path.basename(directory))) {
      await watchDirectory(directory, plugin, onChange, watchers)
    }
  } catch (error) {
    if (error?.code !== 'ENOENT') throw error
  }
}

function isIgnoredPath(root, sourcePath) {
  const relative = path.relative(root, sourcePath)
  return relative.split(path.sep).some((part) => ignoredDirectoryNames.has(part))
}

function quoteGoWorkPath(modulePath) {
  return JSON.stringify(path.resolve(modulePath))
}
