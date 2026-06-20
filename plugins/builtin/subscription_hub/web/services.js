import { normalizePlatform } from './platforms.js'

export const PLATFORM_SERVICE_LABELS = {
  bilibili: {
    all: '全部',
    live: '直播',
    video: '视频',
    image_text: '图文',
    article: '文章',
    repost: '转发',
  },
  weibo: {
    all: '全部',
    post: '微博',
    image: '图片',
    video: '视频',
    repost: '转发',
  },
  douyin: {
    all: '全部',
    video: '视频',
    image_text: '图文',
    live: '直播',
  },
  netease_music: {
    all: '全部',
    song: '歌曲',
    album: '专辑',
    playlist: '歌单',
    artist: '音乐人',
  },
}

export const SERVICE_LABELS = PLATFORM_SERVICE_LABELS.bilibili
export const SERVICE_ORDER = Object.keys(SERVICE_LABELS)
export const SERVICE_TYPES = SERVICE_ORDER.filter((service) => service !== 'all')

export function trim(value) {
  return String(value ?? '').trim()
}

export function unique(values) {
  return [...new Set(values.map(trim).filter(Boolean))]
}

export function serviceLabels(platform = 'bilibili') {
  return PLATFORM_SERVICE_LABELS[normalizePlatform(platform)] || SERVICE_LABELS
}

export function serviceOrder(platform = 'bilibili') {
  return Object.keys(serviceLabels(platform))
}

export function serviceTypes(platform = 'bilibili') {
  return serviceOrder(platform).filter((service) => service !== 'all')
}

export function serviceLabel(service, platform = 'bilibili') {
  return serviceLabels(platform)[service] || service
}

export function normalizeServices(value, platform = 'bilibili') {
  const order = serviceOrder(platform)
  const types = serviceTypes(platform)
  const services = unique(Array.isArray(value) ? value : ['all'])
    .filter((item) => order.includes(item))
  if (!services.length || services.includes('all')) {
    return ['all']
  }
  const selected = types.filter((service) => services.includes(service))
  return selected.length === types.length ? ['all'] : selected
}

export function serviceCheckboxValues(value, platform = 'bilibili') {
  if (Array.isArray(value) && value.length === 0) {
    return new Set()
  }
  const services = normalizeServices(value, platform)
  if (services.includes('all')) {
    return new Set(serviceOrder(platform))
  }
  return new Set(services)
}

export function hasServiceSelection(value) {
  return !(Array.isArray(value) && value.length === 0)
}

export function servicesKey(services, platform = 'bilibili') {
  return normalizeServices(services, platform).join(',')
}

export function servicesText(services, platform = 'bilibili') {
  return normalizeServices(services, platform).map((service) => serviceLabel(service, platform)).join('、')
}
