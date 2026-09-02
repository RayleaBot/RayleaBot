import fs from 'node:fs'
import fsp from 'node:fs/promises'
import path from 'node:path'
import { createFileContentTracker } from './file-content-tracker.mjs'

export const PLUGIN_DEV_OFF = 'off'
export const PLUGIN_DEV_SYNC = 'sync'
export const PLUGIN_DEV_WATCH = 'watch'

const validModes = new Set([PLUGIN_DEV_OFF, PLUGIN_DEV_SYNC, PLUGIN_DEV_WATCH])
const ignoredDirectoryNames = new Set(['.git', '.rayleabot', 'dist', 'node_modules'])
const ignoredDirectoryNamePatterns = [/^_tmp_\d+_[0-9a-f]+$/i]

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
      return { workspaceVersion: '2', plugins: [] }
    }
    throw error
  }
  const document = JSON.parse(raw)
  if (!document || typeof document !== 'object' || Array.isArray(document)
    || document.workspace_version !== '2' || !Array.isArray(document.plugins)) {
    throw new Error(`${workspacePath} does not satisfy plugin development workspace v2.`)
  }
  const allowedRootKeys = new Set(['workspace_version', 'plugins'])
  if (Object.keys(document).some((key) => !allowedRootKeys.has(key))) {
    throw new Error(`${workspacePath} contains an unsupported top-level field.`)
  }
  const workspaceDir = path.dirname(workspacePath)
  const seen = new Set()
  const plugins = []
  for (const [index, entry] of document.plugins.entries()) {
    const allowedKeys = new Set(['path', 'enabled'])
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)
      || Object.keys(entry).some((key) => !allowedKeys.has(key))
      || typeof entry.path !== 'string' || !entry.path.trim()
      || (entry.enabled !== undefined && typeof entry.enabled !== 'boolean')) {
      throw new Error(`${workspacePath} has an invalid plugins[${index}] entry.`)
    }
    if (entry.enabled === false) continue
    const pluginPath = path.resolve(workspaceDir, entry.path)
    let manifest
    try {
      manifest = JSON.parse(await fsp.readFile(path.join(pluginPath, 'info.json'), 'utf8'))
    } catch (error) {
      throw new Error(`${workspacePath} cannot read plugins[${index}] info.json: ${error.message}`)
    }
    const pluginID = manifest?.id
    if (typeof pluginID !== 'string' || !/^[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?$/.test(pluginID)) {
      throw new Error(`${workspacePath} plugins[${index}] info.json has an invalid id.`)
    }
    if (seen.has(pluginID)) {
      throw new Error(`${workspacePath} resolves duplicate plugin id ${pluginID}.`)
    }
    seen.add(pluginID)
    plugins.push({
      id: pluginID,
      path: pluginPath,
      enabled: true,
      hasGoModule: fs.existsSync(path.join(pluginPath, 'go.mod')),
    })
  }
  return { workspaceVersion: '2', plugins }
}

export async function collectWorkspaceSDKVersions(plugins) {
  const versions = new Set()
  for (const plugin of plugins) {
    if (plugin.hasGoModule === false) continue
    const goMod = await fsp.readFile(path.join(plugin.path, 'go.mod'), 'utf8')
    const matches = goMod.matchAll(/github\.com\/RayleaBot\/RayleaBot\/sdk\/go\s+(v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)/g)
    for (const match of matches) {
      versions.add(match[1])
    }
  }
  return [...versions].sort()
}

export function selectWorkspacePlugins(plugins, pluginIDs) {
  if (pluginIDs === undefined || pluginIDs === null) {
    return plugins
  }
  const selectedIDs = new Set(pluginIDs)
  const selected = plugins.filter((plugin) => selectedIDs.has(plugin.id))
  const selectedPluginIDs = new Set(selected.map((plugin) => plugin.id))
  const missing = [...selectedIDs].filter((pluginID) => !selectedPluginIDs.has(pluginID))
  if (missing.length > 0) {
    throw new Error(`Unknown development plugin id(s): ${missing.join(', ')}`)
  }
  return selected
}

export function createDevelopmentReloadQueue() {
  let serverSourcePath = ''
  const pluginChanges = new Map()
  return {
    addServer(sourcePath) {
      serverSourcePath = sourcePath
    },
    addPlugin(plugin, sourcePath) {
      pluginChanges.set(plugin.id, { plugin, sourcePath })
    },
    hasChanges() {
      return serverSourcePath !== '' || pluginChanges.size > 0
    },
    take() {
      const batch = {
        serverSourcePath,
        pluginChanges: [...pluginChanges.values()],
      }
      serverSourcePath = ''
      pluginChanges.clear()
      return batch
    },
  }
}

export function renderDevelopmentGoWork({ sdkGoPath, sdkGoVersions = [], plugins, goVersion = '1.26.6' }) {
  const modulePaths = [sdkGoPath, ...plugins.filter((plugin) => plugin.hasGoModule !== false).map((plugin) => plugin.path)]
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
  const targetNodeModules = path.join(target, 'node_modules')
  const uiNodeModules = path.join(pluginPath, 'ui', 'node_modules')
  const resetUIInstall = fs.existsSync(uiNodeModules) && !fs.existsSync(targetNodeModules)
  await fsp.mkdir(target, { recursive: true })
  const targetEntries = await fsp.readdir(target, { withFileTypes: true })
  await Promise.all(targetEntries
    .filter((entry) => entry.name !== 'node_modules')
    .map((entry) => fsp.rm(path.join(target, entry.name), { recursive: true, force: true })))
  await fsp.cp(sdkVuePath, target, {
    recursive: true,
    filter: (source) => !['node_modules', 'dist'].includes(path.basename(source)),
  })
  if (resetUIInstall) {
    await fsp.rm(uiNodeModules, { recursive: true, force: true })
  }
}

export async function watchPluginWorkspace(plugins, onChange) {
  const watchers = []
  const watchedDirectories = new Set()
  const contentTracker = createFileContentTracker()
  for (const plugin of plugins) {
    await watchDirectory(plugin.path, plugin, onChange, watchers, watchedDirectories, contentTracker)
  }
  return async () => {
    for (const watcher of watchers) {
      watcher.close()
    }
    watchedDirectories.clear()
  }
}

async function watchDirectory(directory, plugin, onChange, watchers, watchedDirectories, contentTracker) {
  const directoryKey = path.resolve(directory)
  if (watchedDirectories.has(directoryKey)) return
  watchedDirectories.add(directoryKey)
  let entries
  try {
    entries = await fsp.readdir(directory, { withFileTypes: true })
  } catch (error) {
    watchedDirectories.delete(directoryKey)
    if (error?.code === 'ENOENT') return
    throw error
  }
  await Promise.all(entries
    .filter((entry) => !entry.isDirectory())
    .map((entry) => contentTracker.prime(path.join(directory, entry.name))))
  const watcher = fs.watch(directory, (eventType, filename) => {
    if (!filename) return
    const sourcePath = path.join(directory, filename.toString())
    void handlePluginWatchEvent({
      eventType,
      sourcePath,
      plugin,
      onChange,
      watchers,
      watchedDirectories,
      contentTracker,
    })
  })
  watchers.push(watcher)
  await Promise.all(entries
    .filter((entry) => entry.isDirectory() && !isIgnoredPath(plugin.path, path.join(directory, entry.name), plugin))
    .map((entry) => watchDirectory(
      path.join(directory, entry.name),
      plugin,
      onChange,
      watchers,
      watchedDirectories,
      contentTracker,
    )))
}

async function handlePluginWatchEvent({
  eventType,
  sourcePath,
  plugin,
  onChange,
  watchers,
  watchedDirectories,
  contentTracker,
}) {
  if (isIgnoredPath(plugin.path, sourcePath, plugin)) return
  const sourceKey = path.resolve(sourcePath)
  try {
    const stat = await fsp.stat(sourcePath)
    if (!stat.isDirectory()) {
      if (await contentTracker.hasChanged(sourcePath)) {
        onChange(plugin, sourcePath)
      }
      return
    }
    const directoryAlreadyWatched = watchedDirectories.has(sourceKey)
    if (eventType === 'rename' && !directoryAlreadyWatched) {
      await watchDirectory(sourcePath, plugin, onChange, watchers, watchedDirectories, contentTracker)
      if (!isNonGoNativeContainer(plugin, sourcePath)) {
        onChange(plugin, sourcePath)
      }
    }
  } catch (error) {
    if (error?.code !== 'ENOENT') throw error
    if (isDirectoryMetadataAlias(sourcePath, sourceKey, watchedDirectories)) return
    const deletedDirectory = watchedDirectories.delete(sourceKey)
    if (deletedDirectory || await contentTracker.hasChanged(sourcePath)) {
      onChange(plugin, sourcePath)
    }
  }
}

function isDirectoryMetadataAlias(sourcePath, sourceKey, watchedDirectories) {
  if (watchedDirectories.has(sourceKey)) return false
  const parentKey = path.resolve(path.dirname(sourcePath))
  return watchedDirectories.has(parentKey) && path.basename(sourcePath) === path.basename(parentKey)
}

function isIgnoredPath(root, sourcePath, plugin) {
  const relative = path.relative(root, sourcePath)
  const parts = relative.split(path.sep)
  for (let index = 0; index < parts.length; index += 1) {
    const part = parts[index]
    if (part === 'dist' && plugin.hasGoModule === false && index === 0) {
      if (parts.length === 1 || parts[1] === 'native') continue
      return true
    }
    if (isIgnoredDirectoryName(part)) return true
  }
  return false
}

function isNonGoNativeContainer(plugin, sourcePath) {
  if (plugin.hasGoModule !== false) return false
  const relative = path.relative(plugin.path, sourcePath)
  return relative === 'dist' || relative === path.join('dist', 'native')
}

function isIgnoredDirectoryName(name) {
  return ignoredDirectoryNames.has(name)
    || ignoredDirectoryNamePatterns.some((pattern) => pattern.test(name))
}

function quoteGoWorkPath(modulePath) {
  return JSON.stringify(path.resolve(modulePath))
}
