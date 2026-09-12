import { t } from '@/i18n'
import { cloneConfig, getConfigSections, getValueByPath, setValueByPath, type ConfigFieldDefinition, type ConfigSectionDefinition } from '@/lib/config-form'
import type { ConfigDocument } from '@/types/api'

export interface ConfigWorkbenchGroup {
  key: string
  title: string
  description: string
  advancedTitle: string
  advancedDescription: string
  sections: ConfigSectionDefinition[]
  commonPaths: string[] | null
}

export function getConfigWorkbenchGroups(): ConfigWorkbenchGroup[] {
  const sections = getConfigSections()
  const definitions = [
    { key: 'access', sections: ['server', 'admin', 'web'], common: ['server.host', 'server.port', 'web.exposure_mode'] },
    { key: 'render', sections: ['render'], common: ['render.default_output', 'render.device_scale_percent', 'render.timeout_seconds'] },
    { key: 'scheduler', sections: ['scheduler'], common: ['scheduler.timezone'] },
    { key: 'runtime', sections: ['runtime', 'http'], common: ['runtime.plugin_event_timeout_seconds', 'runtime.max_concurrent_tasks_per_plugin', 'http.timeout_seconds', 'http.max_retries'] },
    { key: 'data', sections: ['database', 'storage', 'data'], common: ['database.path', 'data.download_cache_retention_days'] },
    { key: 'logs', sections: ['log', 'message'], common: null },
  ]
  return definitions.map(definition => ({
    key: definition.key,
    title: t(`config.workbench.groups.${definition.key}.title`),
    description: t(`config.workbench.groups.${definition.key}.description`),
    advancedTitle: t(`config.workbench.groups.${definition.key}.advancedTitle`),
    advancedDescription: t(`config.workbench.groups.${definition.key}.advancedDescription`),
    sections: definition.sections.flatMap(key => sections.filter(section => section.key === key)),
    commonPaths: definition.common,
  }))
}

export function matchesConfigField(group: ConfigWorkbenchGroup, section: ConfigSectionDefinition, field: ConfigFieldDefinition, query: string) {
  const text = [group.title, section.title, field.label, field.description, field.path].join(' ').toLocaleLowerCase()
  return query.trim().toLocaleLowerCase().split(/\s+/).every(word => text.includes(word))
}

export function getWorkbenchSections(group: ConfigWorkbenchGroup, advanced: boolean, query = '') {
  return group.sections.map(section => ({
    ...section,
    fields: section.fields.filter(field => {
      const isAdvanced = group.commonPaths !== null && !group.commonPaths.includes(field.path)
      return isAdvanced === advanced && matchesConfigField(group, section, field, query)
    }),
  })).filter(section => section.fields.length > 0)
}

export function readConfigField(document: ConfigDocument, path: string) {
  return getValueByPath(document as unknown as Record<string, unknown>, path)
}

export function changedConfigPaths(base: ConfigDocument, draft: ConfigDocument, fields: ConfigFieldDefinition[]) {
  return fields.filter(field => JSON.stringify(readConfigField(base, field.path)) !== JSON.stringify(readConfigField(draft, field.path))).map(field => field.path)
}

// Apply only local field edits to the latest snapshot, retaining values owned by
// other management pages and recording overlapping edits for visible review.
export function rebaseConfigDraft(base: ConfigDocument, draft: ConfigDocument, incoming: ConfigDocument, fields: ConfigFieldDefinition[]) {
  const result = cloneConfig(incoming)
  const conflicts: string[] = []
  for (const path of changedConfigPaths(base, draft, fields)) {
    const value = readConfigField(draft, path)
    const next = readConfigField(incoming, path)
    if (JSON.stringify(next) !== JSON.stringify(readConfigField(base, path)) && JSON.stringify(next) !== JSON.stringify(value)) conflicts.push(path)
    setValueByPath(result as unknown as Record<string, unknown>, path, value === undefined ? undefined : JSON.parse(JSON.stringify(value)))
  }
  return { draft: result, conflicts }
}
