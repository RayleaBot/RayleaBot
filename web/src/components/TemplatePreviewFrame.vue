<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { CSSProperties } from 'vue'

import {
  calculateNativePreviewLayout,
  nativePreviewMinHeight,
  nativePreviewTemplateWidth,
  normalizeNativePreviewFrameWidth,
} from '@/components/template-preview-frame'

const props = withDefaults(defineProps<{
  frameTitle: string
  frameWidth?: number
  srcdoc: string
  templateId: string
  payload?: string
  testIdPrefix?: string
}>(), {
  frameWidth: nativePreviewTemplateWidth,
  payload: '',
  testIdPrefix: 'native-template-preview',
})

const containerRef = ref<HTMLElement | null>(null)
const iframeRef = ref<HTMLIFrameElement | null>(null)
const containerWidth = ref(nativePreviewTemplateWidth)
const containerTop = ref(0)
const contentHeight = ref(nativePreviewMinHeight)
const viewportHeight = ref(typeof window === 'undefined' ? 720 : window.innerHeight)
let resizeObserver: ResizeObserver | null = null
let measureFrame = 0

const previewLayout = computed(() => calculateNativePreviewLayout({
  containerTop: containerTop.value,
  containerWidth: containerWidth.value,
  contentHeight: contentHeight.value,
  viewportHeight: viewportHeight.value,
  frameWidth: normalizedFrameWidth.value,
}))

const normalizedFrameWidth = computed(() => normalizeNativePreviewFrameWidth(props.frameWidth))
const normalizedSrcdoc = computed(() => injectPreviewOverflowGuard(props.srcdoc))
const previewStyle = computed<CSSProperties>(() => ({
  '--native-template-preview-frame-height': `${previewLayout.value.frameHeight}px`,
  '--native-template-preview-frame-width': `${previewLayout.value.frameWidth}px`,
  '--native-template-preview-height': `${previewLayout.value.previewHeight}px`,
  '--native-template-preview-scale': `${previewLayout.value.scale}`,
  '--native-template-preview-scaled-frame-width': `${previewLayout.value.scaledFrameWidth}px`,
}))

const hostTestId = computed(() => `${props.testIdPrefix}-host`)
const frameTestId = computed(() => `${props.testIdPrefix}-frame`)

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

watch(() => [props.srcdoc, props.frameWidth], () => {
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
    const style = typeof window === 'undefined' ? null : window.getComputedStyle(container)
    const horizontalBorderWidth = style
      ? parseFloat(style.borderLeftWidth || '0') + parseFloat(style.borderRightWidth || '0')
      : 0
    const innerWidth = Math.max(0, rect.width - horizontalBorderWidth)
    containerWidth.value = innerWidth > 0 ? innerWidth : nativePreviewTemplateWidth
    containerTop.value = rect.top
  }

  viewportHeight.value = typeof window === 'undefined' ? viewportHeight.value : window.innerHeight
  contentHeight.value = measureFrameContentHeight() || contentHeight.value
}

function measureFrameContentHeight() {
  try {
    const doc = iframeRef.value?.contentDocument
    const surface = doc?.querySelector<HTMLElement>('.surface') ?? doc?.body
    if (!surface) {
      return 0
    }
    return Math.max(surface.scrollHeight, Math.ceil(surface.getBoundingClientRect().height))
  } catch {
    return 0
  }
}

function handleFrameLoad() {
  applyPreviewOverflowGuard()
  queuePreviewMeasure()
  void iframeRef.value?.contentDocument?.fonts?.ready.then(queuePreviewMeasure)
}

function injectPreviewOverflowGuard(srcdoc: string) {
  if (!srcdoc) {
    return srcdoc
  }
  const style = `<style data-rayleabot-preview-guard>html,body{overflow-x:hidden!important;min-width:0!important;max-width:${normalizedFrameWidth.value}px!important;}body{width:${normalizedFrameWidth.value}px!important;}*,*::before,*::after{box-sizing:border-box;}</style>`
  if (srcdoc.includes('data-rayleabot-preview-guard')) {
    return srcdoc
  }
  if (/<\/head>/i.test(srcdoc)) {
    return srcdoc.replace(/<\/head>/i, `${style}</head>`)
  }
  return `${style}${srcdoc}`
}

function applyPreviewOverflowGuard() {
  try {
    const doc = iframeRef.value?.contentDocument
    if (!doc) {
      return
    }
    doc.documentElement.style.overflowX = 'hidden'
    doc.documentElement.style.minWidth = '0'
    doc.documentElement.style.maxWidth = `${normalizedFrameWidth.value}px`
    if (doc.body) {
      doc.body.style.overflowX = 'hidden'
      doc.body.style.minWidth = '0'
      doc.body.style.maxWidth = `${normalizedFrameWidth.value}px`
      doc.body.style.width = `${normalizedFrameWidth.value}px`
    }
  } catch {
    // sandboxed preview documents can become inaccessible while navigating
  }
}
</script>

<template>
  <div
    ref="containerRef"
    class="native-template-preview"
    :style="previewStyle"
    :data-preview-scale="previewLayout.scale.toFixed(4)"
    :data-preview-scrollable="previewLayout.isScrollable ? 'true' : 'false'"
    :data-testid="hostTestId"
  >
    <div class="native-template-preview__scaled-frame">
      <iframe
        ref="iframeRef"
        class="native-template-preview__frame"
        :title="frameTitle"
        sandbox="allow-same-origin"
        :srcdoc="normalizedSrcdoc"
        :data-template-id="templateId"
        :data-preview-payload="payload"
        :data-preview-frame-width="previewLayout.frameWidth"
        :data-preview-frame-height="previewLayout.frameHeight"
        :data-testid="frameTestId"
        @load="handleFrameLoad"
      />
    </div>
  </div>
</template>

<style scoped lang="scss">
.native-template-preview {
  position: relative;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  min-width: 0;
  height: var(--native-template-preview-height);
  min-height: var(--native-template-preview-height);
  overflow: hidden;
  background: var(--surface-soft);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
}

.native-template-preview__scaled-frame {
  flex: 0 0 var(--native-template-preview-scaled-frame-width);
  width: var(--native-template-preview-scaled-frame-width);
  height: var(--native-template-preview-height);
  overflow: hidden;
}

.native-template-preview__frame {
  display: block;
  width: var(--native-template-preview-frame-width);
  max-width: none;
  height: var(--native-template-preview-frame-height);
  border: 0;
  background: transparent;
  transform: scale(var(--native-template-preview-scale));
  transform-origin: center top;
}
</style>
