export const SERVICE_ORDER = ['all', 'live', 'video', 'image_text', 'article', 'repost']
export const SERVICE_TYPES = SERVICE_ORDER.filter((service) => service !== 'all')
export const SERVICE_LABELS = {
  all: '全部',
  live: '直播',
  video: '视频',
  image_text: '图文',
  article: '文章',
  repost: '转发',
}

export function trim(value) {
  return String(value ?? '').trim()
}

export function unique(values) {
  return [...new Set(values.map(trim).filter(Boolean))]
}

export function normalizeServices(value) {
  const services = unique(Array.isArray(value) ? value : ['all'])
    .filter((item) => SERVICE_ORDER.includes(item))
  if (!services.length || services.includes('all')) {
    return ['all']
  }
  const selected = SERVICE_TYPES.filter((service) => services.includes(service))
  return selected.length === SERVICE_TYPES.length ? ['all'] : selected
}

export function serviceCheckboxValues(value) {
  if (Array.isArray(value) && value.length === 0) {
    return new Set()
  }
  const services = normalizeServices(value)
  if (services.includes('all')) {
    return new Set(SERVICE_ORDER)
  }
  return new Set(services)
}

export function hasServiceSelection(value) {
  return !(Array.isArray(value) && value.length === 0)
}

export function servicesKey(services) {
  return normalizeServices(services).join(',')
}

export function servicesText(services) {
  return normalizeServices(services).map((service) => SERVICE_LABELS[service] || service).join('、')
}
