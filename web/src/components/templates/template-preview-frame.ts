export const nativePreviewTemplateWidth = 960
export const nativePreviewMinHeight = 320

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

// The stage fills the height its layout gives it. The frame keeps the rendered content's own height inside
// it, so a short image is not stretched by the template background; taller content is cut to the stage and scrolls.
export function calculateNativePreviewLayout(input: {
  containerWidth: number
  containerHeight: number
  contentHeight: number
  frameWidth?: number
}) {
  const frameWidth = normalizeNativePreviewFrameWidth(input.frameWidth)
  const scale = calculateNativePreviewScale(input.containerWidth, frameWidth)
  const contentHeight = Math.max(nativePreviewMinHeight, Math.ceil(input.contentHeight || nativePreviewMinHeight))
  const scaledFrameWidth = Math.max(1, Math.min(frameWidth, Math.floor(frameWidth * scale)))
  const scaledContentHeight = Math.ceil(contentHeight * scale)
  const previewHeight = Math.max(nativePreviewMinHeight, Math.floor(input.containerHeight || nativePreviewMinHeight))
  const scaledFrameHeight = Math.min(scaledContentHeight, previewHeight)
  const frameHeight = Math.ceil(scaledFrameHeight / scale)

  return {
    contentHeight,
    frameHeight,
    frameWidth,
    isScrollable: contentHeight > frameHeight,
    previewHeight,
    scale,
    scaledContentHeight,
    scaledFrameHeight,
    scaledFrameWidth,
  }
}
