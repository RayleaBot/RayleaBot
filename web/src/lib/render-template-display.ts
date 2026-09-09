import { t } from '@/i18n'

export function getRenderTemplateTypeLabel(type: string) {
  return type.split(' | ').map(part => {
    const key = ['object', 'array', 'string', 'number', 'integer', 'boolean', 'null', 'unknown'].includes(part) ? part : 'unknown'
    return t(`renderTemplates.types.${key}`)
  }).join(' / ')
}
