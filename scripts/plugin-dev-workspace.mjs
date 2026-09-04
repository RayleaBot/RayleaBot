import fs from 'node:fs'
import fsp from 'node:fs/promises'
import path from 'node:path'
import { createFileContentTracker } from './file-content-tracker.mjs'
import { writeIfChanged } from './dev-build-cache.mjs'

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
    || document.workspace_version !== '2' || !Array.isArray(document.plugins) || document.plugins.length > 64) {
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

export function createDevelopmentReloadQueue() {
  let serverSourcePath = ''
  let workspaceSourcePath = ''
  const pluginChanges = new Map()
  return {
    addServer(sourcePath) {
      serverSourcePath = sourcePath
    },
    addWorkspace(sourcePath) {
      workspaceSourcePath = sourcePath
    },
    addPlugin(plugin, sourcePath) {
      if (['info.json', 'go.mod'].includes(path.relative(plugin.path, sourcePath))) {
        workspaceSourcePath = sourcePath
      }
      pluginChanges.set(plugin.id, { plugin, sourcePath })
    },
    hasChanges() {
      return serverSourcePath !== '' || workspaceSourcePath !== '' || pluginChanges.size > 0
    },
    take() {
      const batch = {
        serverSourcePath,
        workspaceSourcePath,
        pluginChanges: [...pluginChanges.values()],
      }
      serverSourcePath = ''
      workspaceSourcePath = ''
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
  async function sync(source, destination) {
    await fsp.mkdir(destination, { recursive: true })
    const entries = (await fsp.readdir(source, { withFileTypes: true }))
      .filter((entry) => !['node_modules', 'dist', '.git'].includes(entry.name))
    const names = new Set(entries.map((entry) => entry.name))
    for (const entry of await fsp.readdir(destination)) {
      if (!names.has(entry) && entry !== 'node_modules') await fsp.rm(path.join(destination, entry), { recursive: true, force: true })
    }
    for (const entry of entries) {
      if (entry.isDirectory()) await sync(path.join(source, entry.name), path.join(destination, entry.name))
      else await writeIfChanged(path.join(destination, entry.name), await fsp.readFile(path.join(source, entry.name)))
    }
  }
  await sync(sdkVuePath, target)
}

export async function watchPluginWorkspace(plugins, onChange, onError = (error) => console.error(error), includesInput = () => false) {
  const watchers = []
  const watchedDirectories = new Map()
  const contentTracker = createFileContentTracker()
  for (const plugin of plugins) {
    await watchDirectory(plugin.path, { ...plugin, includesInput }, onChange, watchers, watchedDirectories, contentTracker, onError)
  }
  return async () => {
    for (const watcher of watchers) {
      watcher.close()
    }
    watchedDirectories.clear()
  }
}

async function watchDirectory(directory, plugin, onChange, watchers, watchedDirectories, contentTracker, onError) {
  const directoryKey = path.resolve(directory)
  if (watchedDirectories.has(directoryKey)) return
  watchedDirectories.set(directoryKey, null)
  let entries
  try {
    entries = await fsp.readdir(directory, { withFileTypes: true })
  } catch (error) {
    watchedDirectories.delete(directoryKey)
    if (error?.code === 'ENOENT') return
    throw error
  }
  await Promise.all(entries
    .filter((entry) => !entry.isDirectory() && !isIgnoredPath(plugin.path, path.join(directory, entry.name), plugin))
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
      onError,
    }).catch(onError)
  })
  watchers.push(watcher)
  watchedDirectories.set(directoryKey, watcher)
  watcher.on('error', onError)
  await Promise.all(entries
    .filter((entry) => entry.isDirectory() && !isIgnoredPath(plugin.path, path.join(directory, entry.name), plugin))
    .map((entry) => watchDirectory(
      path.join(directory, entry.name),
      plugin,
      onChange,
      watchers,
      watchedDirectories,
      contentTracker,
      onError,
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
  onError,
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
      await watchDirectory(sourcePath, plugin, onChange, watchers, watchedDirectories, contentTracker, onError)
      if (!isNonGoNativeContainer(plugin, sourcePath)) {
        onChange(plugin, sourcePath)
      }
    }
  } catch (error) {
    if (error?.code !== 'ENOENT') throw error
    if (isDirectoryMetadataAlias(sourcePath, sourceKey, watchedDirectories)) return
    const deletedDirectory = watchedDirectories.has(sourceKey)
    if (deletedDirectory) {
      for (const [directory, watcher] of watchedDirectories) {
        if (directory === sourceKey || directory.startsWith(sourceKey + path.sep)) {
          watcher?.close()
          watchedDirectories.delete(directory)
        }
      }
    }
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
  if (parts.some((part) => plugin.ignoredDirectories?.includes(part))) return true
  if (plugin.includesInput?.(sourcePath)) return false
  if (parts[0] === '.github') return true
  if (!['assets', 'templates', 'LICENSES'].includes(parts[0]) && /(?:_test\.go|\.(?:test|spec)\.[cm]?[jt]sx?|\.md)$/i.test(relative) && !/^LICENSE|^THIRD_PARTY_NOTICES/.test(parts[0])) return true
  if (parts[0] === 'dist' && parts[1] === 'native' && parts.length > 2 && parts[2] !== currentPluginPlatform()) return true
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
