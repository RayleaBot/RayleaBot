import type { PluginStoreDependency, PluginStoreEntry } from '@/types/api'

// Dependencies of the store release that still have to be installed, in the order the plugin declares them.
export function pendingDependencies(plugin: PluginStoreEntry): PluginStoreDependency[] {
  return (plugin.latest_release?.dependencies ?? []).filter(dependency => dependency.state !== 'installed')
}

// The server refuses the install while any of these is missing, so they can never be skipped.
export function missingRequiredDependencies(plugin: PluginStoreEntry) {
  return pendingDependencies(plugin).filter(dependency => dependency.requirement === 'required')
}
