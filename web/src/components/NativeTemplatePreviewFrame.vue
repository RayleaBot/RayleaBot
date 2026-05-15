<script lang="ts">
export const nativePreviewTemplateWidth = 960
export const nativePreviewMinHeight = 320
export const nativePreviewViewportPadding = 24

export function calculateNativePreviewScale(containerWidth: number) {
  if (!Number.isFinite(containerWidth) || containerWidth <= 0) {
    return 1
  }
  return Math.min(1, containerWidth / nativePreviewTemplateWidth)
}

export function calculateNativePreviewLayout(input: {
  containerWidth: number
  contentHeight: number
  viewportHeight: number
  containerTop: number
}) {
  const scale = calculateNativePreviewScale(input.containerWidth)
  const contentHeight = Math.max(nativePreviewMinHeight, Math.ceil(input.contentHeight || nativePreviewMinHeight))
  const scaledContentHeight = Math.ceil(contentHeight * scale)
  const availableHeight = Math.max(
    nativePreviewMinHeight,
    Math.floor(input.viewportHeight - input.containerTop - nativePreviewViewportPadding),
  )
  const previewHeight = Math.max(nativePreviewMinHeight, Math.min(scaledContentHeight, availableHeight))
  const frameHeight = Math.ceil(previewHeight / scale)

  return {
    availableHeight,
    contentHeight,
    frameHeight,
    isScrollable: contentHeight > frameHeight,
    previewHeight,
    scale,
    scaledContentHeight,
  }
}
</script>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { CSSProperties } from 'vue'

import helpMenuFooterFontUrl from '../../../templates/fortune.card/assets/fonts/lxgw-wenkai-bold/lxgw-wenkai-bold.ttf?url'
import helpMenuStyles from '../../../templates/help.menu/styles.css?raw'

type PreviewData = Record<string, unknown>
type PreviewRecord = Record<string, unknown>

const props = defineProps<{
  templateId: 'help.menu'
  data: PreviewData
}>()

const containerRef = ref<HTMLElement | null>(null)
const iframeRef = ref<HTMLIFrameElement | null>(null)
const containerWidth = ref(nativePreviewTemplateWidth)
const containerTop = ref(0)
const contentHeight = ref(nativePreviewMinHeight)
const viewportHeight = ref(typeof window === 'undefined' ? 720 : window.innerHeight)
let resizeObserver: ResizeObserver | null = null
let measureFrame = 0

const serializedData = computed(() => JSON.stringify(props.data))

const srcdoc = computed(() => buildPreviewDocument(props.templateId, props.data))
const helpMenuPreviewStyles = computed(() => buildHelpMenuPreviewStyles(helpMenuStyles, helpMenuFooterFontUrl))

const previewLayout = computed(() => calculateNativePreviewLayout({
  containerTop: containerTop.value,
  containerWidth: containerWidth.value,
  contentHeight: contentHeight.value,
  viewportHeight: viewportHeight.value,
}))

const previewStyle = computed<CSSProperties>(() => ({
  '--native-template-preview-frame-height': `${previewLayout.value.frameHeight}px`,
  '--native-template-preview-frame-width': `${nativePreviewTemplateWidth}px`,
  '--native-template-preview-height': `${previewLayout.value.previewHeight}px`,
  '--native-template-preview-scale': `${previewLayout.value.scale}`,
}))

onMounted(() => {
  if (typeof window.ResizeObserver === 'function' && containerRef.value) {
    resizeObserver = new window.ResizeObserver(() => queuePreviewMeasure())
    resizeObserver.observe(containerRef.value)
  }

  window.addEventListener('resize', queuePreviewMeasure)
  void nextTick(queuePreviewMeasure)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
  window.removeEventListener('resize', queuePreviewMeasure)
  if (measureFrame) {
    window.cancelAnimationFrame(measureFrame)
    measureFrame = 0
  }
})

watch(srcdoc, () => {
  void nextTick(queuePreviewMeasure)
}, { flush: 'post' })

function queuePreviewMeasure() {
  if (typeof window === 'undefined') {
    measurePreview()
    return
  }

  if (measureFrame) {
    window.cancelAnimationFrame(measureFrame)
  }
  measureFrame = window.requestAnimationFrame(() => {
    measureFrame = 0
    measurePreview()
  })
}

function measurePreview() {
  const container = containerRef.value
  if (container) {
    const rect = container.getBoundingClientRect()
    containerWidth.value = rect.width > 0 ? rect.width : nativePreviewTemplateWidth
    containerTop.value = rect.top
  }

  viewportHeight.value = typeof window === 'undefined' ? viewportHeight.value : window.innerHeight
  contentHeight.value = measureFrameContentHeight() || contentHeight.value
}

function measureFrameContentHeight() {
  try {
    const doc = iframeRef.value?.contentDocument
    const surface = doc?.querySelector<HTMLElement>('.surface')
    if (!surface) {
      return 0
    }
    return Math.max(surface.scrollHeight, Math.ceil(surface.getBoundingClientRect().height))
  } catch {
    return 0
  }
}

function handleFrameLoad() {
  queuePreviewMeasure()
  void iframeRef.value?.contentDocument?.fonts?.ready.then(queuePreviewMeasure)
}

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
        width: ${nativePreviewTemplateWidth}px;
      }
      body {
        overflow: auto;
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
      ${renderIdentity(payload)}
      ${renderHeader(payload)}
      ${body}
      ${renderFooter(payload.render_footer)}
    </main>`
}

function buildHelpMenuPreviewStyles(styles: string, footerFontUrl: string) {
  return styles.replace(
    /url\((["'])\.\.\/fortune\.card\/assets\/fonts\/lxgw-wenkai-bold\/lxgw-wenkai-bold\.ttf\1\)/,
    `url("${escapeCssString(footerFontUrl)}")`,
  )
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
    ? `<img src="${escapeAttribute(avatarUrl)}" alt="" />`
    : `<span>${escapeHtml(displayName)}</span>`
  const titleBadge = userTitle
    ? `<div class="identity-card__badges"><span class="identity-card__title-badge">${escapeHtml(userTitle)}</span></div>`
    : ''

  return `<aside class="identity-card" aria-label="用户身份">
        ${badge}
        <div class="identity-card__avatar">${avatar}</div>
        <div class="identity-card__body">
          <div class="identity-card__name-row">
            <div class="identity-card__name">${escapeHtml(displayName)}</div>
            ${titleBadge}
          </div>
          <div class="identity-card__meta">
            ${optionalElement('span', 'identity-card__meta-line identity-card__meta-line--group', groupName)}
            ${optionalElement('span', 'identity-card__meta-line identity-card__meta-line--id', userId ? `ID ${userId}` : '')}
          </div>
        </div>
      </aside>`
}

function renderHeader(data: PreviewRecord) {
  return `<header class="hero">
        <p class="eyebrow"><span class="eyebrow__dot" aria-hidden="true"></span>RayleaBot</p>
        <h1>${escapeHtml(value(data.title))}</h1>
        ${optionalElement('p', 'subtitle', data.subtitle)}
      </header>`
}

function renderCard(item: unknown) {
  const payload = record(item)
  const level = value(payload.permission)
  const label = permissionLabel(level, payload.permission_label)
  const permission = label
    ? `<span class="command-permission command-permission--${escapeAttribute(level)}">${escapeHtml(label)}</span>`
    : ''

  return `<article class="card">
        <div class="card__accent" aria-hidden="true"></div>
        <div class="card__header">
          <div class="meta">${escapeHtml(value(payload.name, value(payload.title)))}</div>
          ${permission}
        </div>
        ${optionalElement('p', 'description', payload.description)}
        ${optionalElement('code', '', payload.usage)}
      </article>`
}

function renderItemGrid(items: unknown) {
  const cards = Array.isArray(items) ? items.map(renderCard).join('') : ''
  return `<div class="grid">${cards}</div>`
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

function escapeCssString(input: string) {
  return input.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\a ')
}
</script>

<template>
  <div
    ref="containerRef"
    class="native-template-preview"
    :style="previewStyle"
    :data-preview-scale="previewLayout.scale.toFixed(4)"
    :data-preview-scrollable="previewLayout.isScrollable ? 'true' : 'false'"
    data-testid="native-template-preview-host"
  >
    <iframe
      ref="iframeRef"
      class="native-template-preview__frame"
      title="help.menu"
      sandbox="allow-same-origin"
      :srcdoc="srcdoc"
      :data-template-id="templateId"
      :data-preview-payload="serializedData"
      :data-preview-frame-width="nativePreviewTemplateWidth"
      :data-preview-frame-height="previewLayout.frameHeight"
      data-testid="native-template-preview-frame"
      @load="handleFrameLoad"
    />
  </div>
</template>

<style scoped lang="scss">
.native-template-preview {
  position: relative;
  min-width: 0;
  height: var(--native-template-preview-height);
  min-height: var(--native-template-preview-height);
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: #111827;
}

.native-template-preview__frame {
  display: block;
  width: var(--native-template-preview-frame-width);
  height: var(--native-template-preview-frame-height);
  border: 0;
  background: transparent;
  transform: scale(var(--native-template-preview-scale));
  transform-origin: left top;
}
</style>
