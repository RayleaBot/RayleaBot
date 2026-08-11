<script lang="ts">
export {
  calculateNativePreviewLayout,
  calculateNativePreviewScale,
  nativePreviewMinHeight,
  nativePreviewTemplateWidth,
  nativePreviewViewportPadding,
} from '@/components/template-preview-frame'

export function stripHelpMenuPreviewFontImports(styles: string) {
  return styles
    .replace(/@import\s+url\(["']\.\.\/fortune\.card\/assets\/fonts\/lxgwwenkai-medium\/result\.css["']\);?\s*/g, '')
    .replace(/@import\s+url\(["']\.\.\/fortune\.card\/assets\/fonts\/lxgw-wenkai-medium\/result\.css["']\);?\s*/g, '')
}
</script>

<script setup lang="ts">
import { computed } from 'vue'

import TemplatePreviewFrame from '@/components/TemplatePreviewFrame.vue'
import helpMenuStyles from '../../../templates/help.menu/styles.css?raw'

type PreviewData = Record<string, unknown>
type PreviewRecord = Record<string, unknown>
type PreviewUsagePartKind = 'literal' | 'required' | 'optional'

interface PreviewUsagePart {
  kind: PreviewUsagePartKind
  text: string
}

const props = defineProps<{
  templateId: 'help.menu'
  data: PreviewData
}>()

const serializedData = computed(() => JSON.stringify(props.data))
const srcdoc = computed(() => buildPreviewDocument(props.templateId, props.data))
const helpMenuPreviewStyles = computed(() => stripHelpMenuPreviewFontImports(helpMenuStyles))

function buildPreviewDocument(templateId: string, data: PreviewData) {
  if (templateId !== 'help.menu') {
    return ''
  }

  return `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style>${helpMenuPreviewStyles.value}</style>
    <style>
      html, body {
        min-height: 100%;
        width: 100%;
        max-width: 100%;
      }
      body {
        overflow-x: hidden;
        overflow-y: auto;
      }
    </style>
  </head>
  <body class="theme-default">
    ${renderHelpMenu(data)}
  </body>
</html>`
}

function renderHelpMenu(data: PreviewData) {
  const payload = record(data)
  const body = Array.isArray(payload.groups) && payload.groups.length > 0
    ? renderGroups(payload.groups)
    : renderItemGrid(payload.items)

  return `<main class="surface" id="preview-root">
      <header class="page-header">
        ${renderTitleArea(payload)}
        ${renderIdentity(payload)}
      </header>
      ${renderCommandGuide(payload)}
      ${body}
      ${renderFooter(payload.render_footer)}
    </main>`
}

function renderIdentity(data: PreviewRecord) {
  const user = record(data.user)
  const group = record(data.group)
  const permission = record(data.permission)
  const userId = value(user.id)
  const displayName = value(user.nickname, userId || '访客')
  const avatarUrl = value(user.avatar_url)
  const userTitle = value(user.title)
  const groupName = value(group.name)
  const permissionLevel = value(permission.level, 'member')
  const showPermissionBadge = Boolean(groupName) || permissionLevel === 'super_admin'
  const badge = showPermissionBadge ? renderPermissionBadge(permissionLevel) : ''
  const avatar = avatarUrl
    ? `<img src="${escapeAttribute(avatarUrl)}" alt="" class="identity-avatar" />`
    : `<div class="identity-avatar identity-avatar--fallback"><span>${escapeHtml(displayName)}</span></div>`
  const titleEl = userTitle
    ? `<span class="identity-title">${escapeHtml(userTitle)}</span>`
    : ''
  const groupEl = groupName
    ? `<span>${escapeHtml(groupName)}</span>`
    : ''
  const idEl = userId
    ? `<span>ID ${escapeHtml(userId)}</span>`
    : ''
  const metaEl = groupEl || idEl
    ? `<div class="identity-meta">${groupEl}${idEl}</div>`
    : ''

  return `<div class="page-header__identity">
        <div class="identity-avatar-wrap">
          ${avatar}
        </div>
        <div class="identity-info">
          <div class="identity-name-row">
            <span class="identity-name">${escapeHtml(displayName)}</span>
            ${badge}
          </div>
          ${titleEl}
          ${metaEl}
        </div>
      </div>`
}

function renderTitleArea(data: PreviewRecord) {
  return `<div class="page-header__title-area">
        <h1>${escapeHtml(value(data.title))}</h1>
        ${optionalElement('p', 'subtitle', data.subtitle)}
        ${renderCommandPrefixes(data.command_prefixes)}
      </div>`
}

function renderCommandPrefixes(input: unknown) {
  const prefixes = stringList(input)
  if (prefixes.length === 0) {
    return ''
  }
  return `<div class="command-prefixes" aria-label="可用指令前缀">
        <span class="command-prefixes__label">前缀</span>
        <span class="command-prefixes__values">
          ${prefixes.map((prefix) => `<code>${escapeHtml(prefix)}</code>`).join('')}
        </span>
      </div>`
}

function renderCommandGuide(data: PreviewRecord) {
  const examples = stringList(data.trigger_examples)
  if (examples.length === 0) {
    return ''
  }
  return `<section class="command-guide" aria-label="菜单触发方式">
        <div class="command-guide__block command-guide__block--examples">
          <span class="command-guide__label">触发指令示例</span>
          <div class="command-guide__chips">
            ${examples.map((item) => `<code>${escapeHtml(item)}</code>`).join('')}
          </div>
        </div>
      </section>`
}

function renderCell(item: unknown) {
  const payload = record(item)
  const level = value(payload.permission)
  const label = permissionLabel(level, payload.permission_label)
  const permission = label
    ? `<span class="command-permission command-permission--${escapeAttribute(level)}">${escapeHtml(label)}</span>`
    : ''

  return `<article class="cell">
        <div class="cell__header">
          <div class="meta">${escapeHtml(value(payload.name, value(payload.title)))}</div>
          ${permission}
        </div>
        ${optionalElement('p', 'description', payload.description)}
        ${renderCommandUsage(payload)}
      </article>`
}

function renderCommandUsage(payload: PreviewRecord) {
  const name = value(payload.name, value(payload.title))
  const commandSource = value(payload.command_source)
  const usage = value(payload.usage)
  const usageArgs = value(payload.usage_args)
  const prefixes = stringList(payload.command_prefixes)
  const commandName = commandSource === 'pattern' ? usage : name
  if (!commandName || prefixes.length === 0) {
    return ''
  }
  const parts = previewUsageParts(payload.usage_parts)
  const content = commandSource === 'pattern'
    ? parts.length > 0
      ? renderUsageParts(parts)
      : `<span class="command-usage__name">${escapeHtml(commandName)}</span>`
    : `<span class="command-usage__name">${escapeHtml(commandName)}</span>${parts.length > 0
      ? renderUsageParts(parts)
      : usageArgs
        ? `<span class="command-usage__args">${escapeHtml(usageArgs)}</span>`
        : ''}`
  return `<div class="command-usage" aria-label="指令示意">
        <code>${content}</code>
      </div>`
}

function previewUsageParts(input: unknown): PreviewUsagePart[] {
  if (!Array.isArray(input)) {
    return []
  }
  const parts: PreviewUsagePart[] = []
  for (const item of input) {
    const payload = record(item)
    const kind = value(payload.kind)
    const text = value(payload.text)
    if ((kind === 'literal' || kind === 'required' || kind === 'optional') && text) {
      parts.push({ kind, text })
    }
  }
  return parts
}

function renderUsageParts(parts: PreviewUsagePart[]) {
  return parts.map((part) => {
    if (part.kind === 'literal') {
      return `<span class="command-usage__name">${escapeHtml(part.text)}</span>`
    }
    const label = part.kind === 'optional' ? '可选' : '必填'
    return `<span class="command-argument command-argument--${part.kind}" aria-label="${label}参数 ${escapeAttribute(part.text)}"><span class="command-argument__label">${label}</span><span class="command-argument__value">${escapeHtml(part.text)}</span></span>`
  }).join('')
}

function renderItemGrid(items: unknown) {
  const cells = Array.isArray(items) ? items.map(renderCell).join('') : ''
  return `<div class="grid">${cells}</div>`
}

function renderGroups(groups: unknown) {
  if (!Array.isArray(groups)) {
    return ''
  }

  return groups
    .map((group) => {
      const payload = record(group)
      return `<section class="help-group">
        <h2><span class="help-group__marker" aria-hidden="true"></span><span class="help-group__title">${escapeHtml(value(payload.title))}</span></h2>
        ${renderItemGrid(payload.items)}
      </section>`
    })
    .join('')
}

function renderFooter(input: unknown) {
  const content = value(input)
  if (!content) {
    return ''
  }
  return `<footer class="template-footer"><span class="template-footer__text">${escapeHtml(content)}</span></footer>`
}

function renderPermissionBadge(level: string) {
  const badgeClass = level === 'super_admin'
    ? 'permission-badge--super-admin'
    : level === 'owner'
      ? 'permission-badge--owner'
      : level === 'admin'
        ? 'permission-badge--admin'
        : 'permission-badge--member'
  const label = level === 'super_admin'
    ? '超级管理员'
    : level === 'owner'
      ? '群主'
      : level === 'admin'
        ? '管理员'
        : '群员'

  return `<span class="permission-badge ${badgeClass}">${label}</span>`
}

function permissionLabel(level: string, explicitLabel: unknown) {
  const label = value(explicitLabel)
  if (label) return label
  if (level === 'super_admin') return '超级管理员'
  if (level === 'group_admin') return '群管理员'
  if (level === 'everyone') return '所有人'
  return ''
}

function optionalElement(tag: string, className: string, input: unknown) {
  const content = value(input)
  if (!content) {
    return ''
  }
  const classAttribute = className ? ` class="${escapeAttribute(className)}"` : ''
  return `<${tag}${classAttribute}>${escapeHtml(content)}</${tag}>`
}

function record(input: unknown): PreviewRecord {
  return input && typeof input === 'object' && !Array.isArray(input) ? input as PreviewRecord : {}
}

function value(input: unknown, fallback = '') {
  if (input === undefined || input === null) return fallback
  const text = String(input).trim()
  return text || fallback
}

function stringList(input: unknown) {
  if (!Array.isArray(input)) {
    return []
  }
  const values: string[] = []
  for (const item of input) {
    const text = value(item)
    if (text) {
      values.push(text)
    }
  }
  return values
}

function escapeHtml(input: string) {
  return input
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function escapeAttribute(input: string) {
  return escapeHtml(input).replace(/`/g, '&#96;')
}

</script>

<template>
  <TemplatePreviewFrame
    frame-title="help.menu"
    :srcdoc="srcdoc"
    :template-id="templateId"
    :payload="serializedData"
  />
</template>
