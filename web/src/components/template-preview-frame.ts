export const nativePreviewTemplateWidth = 960
export const nativePreviewMinHeight = 320
export const nativePreviewViewportPadding = 24

export function normalizeNativePreviewFrameWidth(frameWidth?: number) {
  if (!Number.isFinite(frameWidth) || !frameWidth || frameWidth <= 0) {
    return nativePreviewTemplateWidth
  }
  return Math.ceil(frameWidth)
}

export function calculateNativePreviewScale(containerWidth: number, frameWidth = nativePreviewTemplateWidth) {
  if (!Number.isFinite(containerWidth) || containerWidth <= 0) {
    return 1
  }
  return Math.min(1, containerWidth / normalizeNativePreviewFrameWidth(frameWidth))
}

export function calculateNativePreviewLayout(input: {
  containerWidth: number
  contentHeight: number
  viewportHeight: number
  containerTop: number
  frameWidth?: number
}) {
  const frameWidth = normalizeNativePreviewFrameWidth(input.frameWidth)
  const scale = calculateNativePreviewScale(input.containerWidth, frameWidth)
  const contentHeight = Math.max(nativePreviewMinHeight, Math.ceil(input.contentHeight || nativePreviewMinHeight))
  const scaledFrameWidth = Math.max(1, Math.min(frameWidth, Math.floor(frameWidth * scale)))
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
    frameWidth,
    isScrollable: contentHeight > frameHeight,
    previewHeight,
    scale,
    scaledContentHeight,
    scaledFrameWidth,
  }
}
