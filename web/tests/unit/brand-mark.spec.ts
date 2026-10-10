import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import RayleaMark from '@/components/brand/RayleaMark.vue'

describe('brand artwork selection', () => {
  it.each([32, 40] as const)('uses compact artwork for every density at %spx', size => {
    const wrapper = mount(RayleaMark, { props: { size } })
    for (const image of wrapper.findAll('img')) {
      expect(image.attributes('src')).toContain('mark-compact-')
      expect(image.attributes('sizes')).toBe(`${size}px`)
      for (const candidate of image.attributes('srcset').split(', ')) expect(candidate).toContain('mark-compact-')
    }
  })

  it('switches to portrait artwork when the display size becomes large', async () => {
    const wrapper = mount(RayleaMark, { props: { size: 40 } })
    await wrapper.setProps({ size: 128 })
    for (const image of wrapper.findAll('img')) {
      expect(image.attributes('src')).toMatch(/mark-(light|dark)-128\.png/)
      expect(image.attributes('srcset')).not.toContain('mark-compact-')
      expect(image.attributes('sizes')).toBe('128px')
    }
  })
})
