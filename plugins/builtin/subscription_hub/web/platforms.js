export const PLATFORM_OPTIONS = [
  { value: 'bilibili', label: 'Bilibili', subjectLabel: 'UID', inputPlaceholder: 'UID 或 Bilibili 用户名' },
  { value: 'weibo', label: '微博', subjectLabel: 'UID', inputPlaceholder: 'UID 或微博主页标识' },
  { value: 'douyin', label: '抖音', subjectLabel: '抖音号', inputPlaceholder: '抖音号或主页标识' },
  { value: 'netease_music', label: '网易云音乐', subjectLabel: 'ID', inputPlaceholder: '歌曲、歌单、专辑或音乐人 ID' },
]

const PLATFORM_MAP = new Map(PLATFORM_OPTIONS.map((item) => [item.value, item]))

export function normalizePlatform(value) {
  const platform = String(value ?? '').trim()
  return PLATFORM_MAP.has(platform) ? platform : 'bilibili'
}

export function platformMeta(platform) {
  return PLATFORM_MAP.get(normalizePlatform(platform)) || PLATFORM_OPTIONS[0]
}

export function platformLabel(platform) {
  return platformMeta(platform).label
}

export function subjectLabel(platform) {
  return platformMeta(platform).subjectLabel
}

export function inputPlaceholder(platform) {
  return platformMeta(platform).inputPlaceholder
}

export function safeSubjectId(value, platform = 'bilibili') {
  const text = String(value ?? '').trim()
  if (normalizePlatform(platform) === 'bilibili') {
    return /^[0-9]+$/.test(text) ? text : ''
  }
  return [...text]
    .filter((char) => /[\p{L}\p{N}_.-]/u.test(char))
    .join('')
    .replace(/^[_.-]+|[_.-]+$/g, '')
    .slice(0, 96)
}

export function subjectText(row) {
  const name = String(row?.name ?? '').trim()
  const uid = String(row?.uid ?? '').trim()
  const label = subjectLabel(row?.platform)
  return name && uid && name !== uid ? `${name}（${label} ${uid}）` : uid || name
}
