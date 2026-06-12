import { trim } from '../services.js'

export function escapeHTML(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

export function selectorValue(value) {
  return String(value ?? '').replaceAll('\\', '\\\\').replaceAll('"', '\\"')
}

function generateHueFromString(value) {
  let hash = 0
  const text = trim(value) || '?'
  for (let i = 0; i < text.length; i += 1) {
    hash = text.charCodeAt(i) + ((hash << 5) - hash)
  }
  return Math.abs(hash) % 360
}

export function avatarHTML(avatarUrl, fallbackText, sizeClass, alt) {
  const hue = generateHueFromString(fallbackText || '?')
  const bg = `hsl(${hue} 72% 58%)`
  const text = (fallbackText || '?').slice(0, 1).toUpperCase()
  const safeBg = escapeHTML(bg)
  const safeText = escapeHTML(text)
  const safeAlt = escapeHTML(alt || '')
  const safeUrl = escapeHTML(avatarUrl || '')
  const safeSize = escapeHTML(sizeClass)
  return `
    <span class="avatar ${safeSize}" style="background:${safeBg}" aria-label="${safeAlt}">
      <img src="${safeUrl}" alt="${safeAlt}" loading="lazy" referrerpolicy="no-referrer" onerror="this.style.display='none'; this.parentNode.querySelector('.avatar-fallback__text').style.display='flex'" />
      <span class="avatar-fallback__text">${safeText}</span>
    </span>
  `
}

export function avatarStackHTML(items, maxVisible, sizeClass, getAvatar, getLabel) {
  if (!items.length) {
    return '<span class="sub-card__summary-label">无</span>'
  }
  const visible = items.slice(0, maxVisible)
  const overflow = items.length - visible.length
  const avatars = visible.map((item) => avatarHTML(getAvatar(item), getLabel(item), sizeClass, getLabel(item))).join('')
  const overflowHTML = overflow > 0 ? `<span class="avatar-stack__overflow">+${overflow}</span>` : ''
  return `<span class="avatar-stack">${avatars}${overflowHTML}</span>`
}
