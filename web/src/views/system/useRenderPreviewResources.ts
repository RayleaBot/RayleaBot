import { ref } from 'vue'

import { useRenderTemplatesStore } from '@/stores/render-templates'
import type { RenderTemplatePreviewHTMLResponse } from '@/types/api'

export interface PreviewDocumentState extends RenderTemplatePreviewHTMLResponse {
  cacheKey: string
  resourceKeys: string[]
}

interface PreviewResourceCacheEntry {
  blobUrl: string
  refCount: number
}

interface PreviewResourceTextCacheEntry {
  text: string
}

interface PreviewResourceRewriteContext {
  createdResourceKeys: string[]
  resourceKeys: string[]
  resolved: Map<string, Promise<string>>
  resolvedText: Map<string, Promise<string>>
  signal: AbortSignal
  templateId: string
}

export function useRenderPreviewResources(renderTemplatesStore: ReturnType<typeof useRenderTemplatesStore>) {
  const previewDocumentByTemplate = ref<Record<string, PreviewDocumentState>>({})
  const previewDocumentCache = new Map<string, PreviewDocumentState>()
  const previewResourceCache = new Map<string, PreviewResourceCacheEntry>()
  const previewResourceTextCache = new Map<string, PreviewResourceTextCacheEntry>()

  function revokePreviewDocument(templateId: string) {
    if (!(templateId in previewDocumentByTemplate.value)) {
      return
    }
    const next = { ...previewDocumentByTemplate.value }
    delete next[templateId]
    previewDocumentByTemplate.value = next
  }

  function retainPreviewDocumentResources(document: PreviewDocumentState) {
    for (const cacheKey of document.resourceKeys) {
      const entry = previewResourceCache.get(cacheKey)
      if (entry) {
        entry.refCount += 1
      }
    }
  }

  function releasePreviewDocumentResources(document: PreviewDocumentState) {
    releasePreviewResourceKeys(document.resourceKeys, { force: false })
  }

  function releasePreviewResourceKeys(cacheKeys: string[], options: { force: boolean }) {
    for (const cacheKey of new Set(cacheKeys)) {
      const entry = previewResourceCache.get(cacheKey)
      if (!entry) {
        continue
      }
      if (!options.force) {
        entry.refCount -= 1
      }
      if (options.force || entry.refCount <= 0) {
        previewResourceCache.delete(cacheKey)
        window.URL.revokeObjectURL(entry.blobUrl)
      }
    }
  }

  function clearPreviewDocumentCaches() {
    const released = new Set<string>()
    for (const document of Object.values(previewDocumentByTemplate.value)) {
      if (released.has(document.cacheKey)) {
        continue
      }
      released.add(document.cacheKey)
      releasePreviewDocumentResources(document)
    }
    for (const [cacheKey, document] of previewDocumentCache) {
      if (released.has(cacheKey)) {
        continue
      }
      released.add(cacheKey)
      releasePreviewDocumentResources(document)
    }
    previewDocumentCache.clear()
    previewDocumentByTemplate.value = {}
    previewResourceTextCache.clear()
    for (const cacheKey of Array.from(previewResourceCache.keys())) {
      releasePreviewResourceKeys([cacheKey], { force: true })
    }
  }

  async function rewritePreviewDocumentResources(templateId: string, html: string, revisionId: string, signal: AbortSignal) {
    const createdResourceKeys: string[] = []
    const resourceKeys: string[] = []
    const context: PreviewResourceRewriteContext = {
      createdResourceKeys,
      resourceKeys,
      resolved: new Map(),
      resolvedText: new Map(),
      signal,
      templateId,
    }
    const document = new DOMParser().parseFromString(html, 'text/html')

    try {
      await Promise.all([
        ...Array.from(document.querySelectorAll('style')).map(async (style) => {
          style.textContent = await rewriteCSSResources(style.textContent ?? '', '', revisionId, context)
        }),
        ...Array.from(document.querySelectorAll<HTMLElement>('[style]')).map(async (element) => {
          const style = element.getAttribute('style') ?? ''
          element.setAttribute('style', await rewriteCSSResources(style, '', revisionId, context))
        }),
        ...Array.from(document.querySelectorAll<HTMLElement>('[src]')).map((element) => (
          rewriteElementResourceAttribute(element, 'src', '', revisionId, context)
        )),
        ...Array.from(document.querySelectorAll<HTMLLinkElement>('link[href]')).map((link) => (
          rewriteLinkResource(link, document, revisionId, context)
        )),
      ])
    } catch (error) {
      releasePreviewResourceKeys(createdResourceKeys, { force: true })
      throw error
    }

    return {
      createdResourceKeys,
      html: `<!doctype html>\n${document.documentElement.outerHTML}`,
      resourceKeys,
    }
  }

  async function rewriteElementResourceAttribute(
    element: HTMLElement,
    attribute: string,
    basePath: string,
    revisionId: string,
    context: PreviewResourceRewriteContext,
  ) {
    const raw = element.getAttribute(attribute) ?? ''
    const resourcePath = resolvePreviewResourcePath(basePath, raw)
    if (!resourcePath) {
      return
    }

    const blobUrl = await downloadTemplateAssetObjectURL(resourcePath, revisionId, context)
    element.setAttribute(attribute, blobUrl)
  }

  async function rewriteLinkResource(
    link: HTMLLinkElement,
    document: Document,
    revisionId: string,
    context: PreviewResourceRewriteContext,
  ) {
    const rel = (link.getAttribute('rel') ?? '').toLowerCase()
    const href = link.getAttribute('href') ?? ''
    const resourcePath = resolvePreviewResourcePath('', href)
    if (!resourcePath) {
      return
    }

    if (rel.includes('stylesheet')) {
      const css = await downloadTemplateAssetText(resourcePath, revisionId, context)
      const style = document.createElement('style')
      style.textContent = await rewriteCSSResources(css, dirname(resourcePath), revisionId, context)
      link.replaceWith(style)
      return
    }

    const blobUrl = await downloadTemplateAssetObjectURL(resourcePath, revisionId, context)
    link.setAttribute('href', blobUrl)
  }

  async function rewriteCSSResources(css: string, basePath: string, revisionId: string, context: PreviewResourceRewriteContext): Promise<string> {
    let rewritten = css

    rewritten = await replaceAsync(rewritten, /@import\s+(?:url\()?["']?([^"')\s;]+)["']?\)?[^;]*;?/gi, async (match, rawUrl: string) => {
      const resourcePath = resolvePreviewResourcePath(basePath, rawUrl)
      if (!resourcePath) {
        return match
      }
      const importedCSS = await downloadTemplateAssetText(resourcePath, revisionId, context)
      return rewriteCSSResources(importedCSS, dirname(resourcePath), revisionId, context)
    })

    rewritten = await replaceAsync(rewritten, /url\(\s*(["']?)([^"')]+)\1\s*\)/gi, async (match, quote: string, rawUrl: string) => {
      const resourcePath = resolvePreviewResourcePath(basePath, rawUrl)
      if (!resourcePath) {
        return match
      }
      const blobUrl = await downloadTemplateAssetObjectURL(resourcePath, revisionId, context)
      return `url(${quote}${blobUrl}${quote})`
    })

    return rewritten
  }

  async function downloadTemplateAssetText(path: string, revisionId: string, context: PreviewResourceRewriteContext) {
    const cacheKey = `${context.templateId}:${revisionId}:${path}`
    const existing = previewResourceTextCache.get(cacheKey)
    if (existing) {
      return existing.text
    }
    const pending = context.resolvedText.get(cacheKey)
    if (pending) {
      return pending
    }

    const promise = renderTemplatesStore.downloadTemplateAsset(context.templateId, path, context.signal)
      .then(async ({ blob }) => {
        const text = await readPreviewResourceText(blob)
        previewResourceTextCache.set(cacheKey, { text })
        return text
      })
    context.resolvedText.set(cacheKey, promise)
    return promise
  }

  async function readPreviewResourceText(blob: Blob) {
    if (typeof blob.text === 'function') {
      return blob.text()
    }
    return new Response(blob).text()
  }

  async function downloadTemplateAssetObjectURL(path: string, revisionId: string, context: PreviewResourceRewriteContext) {
    const cacheKey = `${context.templateId}:${revisionId}:${path}`
    const existing = previewResourceCache.get(cacheKey)
    if (existing) {
      addPreviewResourceKey(context, cacheKey)
      return existing.blobUrl
    }
    const pending = context.resolved.get(cacheKey)
    if (pending) {
      const blobUrl = await pending
      addPreviewResourceKey(context, cacheKey)
      return blobUrl
    }

    const promise = renderTemplatesStore.downloadTemplateAsset(context.templateId, path, context.signal)
      .then(({ blob }) => {
        const blobUrl = window.URL.createObjectURL(blob)
        previewResourceCache.set(cacheKey, { blobUrl, refCount: 0 })
        addCreatedPreviewResourceKey(context, cacheKey)
        return blobUrl
      })
    context.resolved.set(cacheKey, promise)
    const blobUrl = await promise
    addPreviewResourceKey(context, cacheKey)
    return blobUrl
  }

  function addPreviewResourceKey(context: PreviewResourceRewriteContext, cacheKey: string) {
    if (!context.resourceKeys.includes(cacheKey)) {
      context.resourceKeys.push(cacheKey)
    }
  }

  function addCreatedPreviewResourceKey(context: PreviewResourceRewriteContext, cacheKey: string) {
    if (!context.createdResourceKeys.includes(cacheKey)) {
      context.createdResourceKeys.push(cacheKey)
    }
  }

  function resolvePreviewResourcePath(basePath: string, rawUrl: string) {
    const url = stripResourceURL(rawUrl)
    if (!url || isExternalPreviewResource(url)) {
      return ''
    }
    return normalizePreviewResourcePath(basePath ? `${basePath}/${url}` : url)
  }

  function stripResourceURL(rawUrl: string) {
    return String(rawUrl ?? '').trim().replace(/^["']|["']$/g, '').split(/[?#]/)[0]
  }

  function isExternalPreviewResource(url: string) {
    return url.startsWith('#')
      || url.startsWith('/')
      || /^[a-z][a-z0-9+.-]*:/i.test(url)
  }

  function dirname(path: string) {
    const normalized = normalizePreviewResourcePath(path)
    const index = normalized.lastIndexOf('/')
    return index > 0 ? normalized.slice(0, index) : ''
  }

  function normalizePreviewResourcePath(path: string) {
    const segments: string[] = []
    for (const segment of path.replace(/\\/g, '/').split('/')) {
      if (!segment || segment === '.') {
        continue
      }
      if (segment === '..') {
        if (segments.length > 0 && segments[segments.length - 1] !== '..') {
          segments.pop()
        } else {
          segments.push(segment)
        }
        continue
      }
      segments.push(segment)
    }
    return segments.join('/')
  }

  async function replaceAsync(source: string, pattern: RegExp, replacer: (...args: any[]) => Promise<string>) {
    const matches = Array.from(source.matchAll(pattern))
    if (matches.length === 0) {
      return source
    }

    const replacements = await Promise.all(matches.map((match) => replacer(...match)))
    let result = ''
    let lastIndex = 0
    matches.forEach((match, index) => {
      result += source.slice(lastIndex, match.index)
      result += replacements[index]
      lastIndex = (match.index ?? 0) + match[0].length
    })
    result += source.slice(lastIndex)
    return result
  }

  return {
    clearPreviewDocumentCaches,
    previewDocumentByTemplate,
    previewDocumentCache,
    releasePreviewDocumentResources,
    releasePreviewResourceKeys,
    retainPreviewDocumentResources,
    revokePreviewDocument,
    rewritePreviewDocumentResources,
  }
}
