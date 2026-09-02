import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import NativeTemplatePreviewFrame, {
  nativePreviewTemplateWidth,
  rewriteHelpMenuPreviewFontSources,
  stripHelpMenuPreviewFontImports,
} from '@/components/NativeTemplatePreviewFrame.vue'
import helpMenuFontFaces from '../../../templates/help.menu/assets/fonts/noto-sans-sc/result.css?raw'
import helpMenuStyles from '../../../templates/help.menu/styles.css?raw'

describe('NativeTemplatePreviewFrame', () => {
  it('uses managed font assets without preserving external imports or ttf sources', () => {
    const styles = stripHelpMenuPreviewFontImports(helpMenuStyles)
    const fontFaces = rewriteHelpMenuPreviewFontSources(helpMenuFontFaces)

    expect(styles).not.toContain('assets/fonts/noto-sans-sc/result.css')
    expect(fontFaces).toContain('@font-face')
    expect(fontFaces).toMatch(/\/api\/system\/render\/templates\/help\.menu\/asset\?path=assets%2Ffonts%2Fnoto-sans-sc%2F[A-Za-z0-9._-]+\.woff2/)
    expect(fontFaces).not.toMatch(/url\(\s*["']?\.\//)
    expect(fontFaces).not.toMatch(/\.ttf\b/i)
  })

  it('escapes preview text and attributes before writing srcdoc', () => {
    const wrapper = mount(NativeTemplatePreviewFrame, {
      props: {
        templateId: 'help.menu',
        data: {
          title: '<script>alert(1)</script>',
          user: {
            nickname: '<Admin>',
            avatar_url: 'https://example.test/avatar.png" onerror="alert(2)',
          },
          items: [{
            name: 'echo"><img src=x onerror=alert(3)>',
            description: '<b>unsafe</b>',
          }],
        },
      },
    })

    const frame = wrapper.get('[data-testid="native-template-preview-frame"]')
    const srcdoc = frame.attributes('srcdoc') ?? ''
    const document = new DOMParser().parseFromString(srcdoc, 'text/html')

    expect(document.querySelector('script')).toBeNull()
    expect(document.querySelector('[onerror]')).toBeNull()
    expect(document.querySelector('h1')?.textContent).toBe('<script>alert(1)</script>')
    expect(document.querySelector('.description')?.textContent).toBe('<b>unsafe</b>')
    expect(document.querySelectorAll('img')).toHaveLength(1)
  })

  it('keeps the preview sandboxed and reports canonical frame bounds', () => {
    const wrapper = mount(NativeTemplatePreviewFrame, {
      props: {
        templateId: 'help.menu',
        data: { title: 'Plugin menu', items: [] },
      },
    })

    const frame = wrapper.get('[data-testid="native-template-preview-frame"]')
    expect(frame.attributes('sandbox')).toBe('allow-same-origin')
    expect(frame.attributes('data-preview-frame-width')).toBe(String(nativePreviewTemplateWidth))
    expect(Number(frame.attributes('data-preview-frame-height'))).toBeGreaterThan(0)
  })
})
