export interface FortuneItem {
  name: string
  stars: string
  sign: string
  explanation: string
}

export interface SpecialDate {
  date: string
  fortune_name: string
}

export interface FortuneSettings {
  trigger_commands: string[]
  stats_trigger_commands: string[]
  timezone: string
  fortunes: FortuneItem[]
  special_dates: SpecialDate[]
  good_actions: string[]
  bad_actions: string[]
}

export function normalizeStringList(value: unknown, fallback: string[] = []): string[] {
  const source = Array.isArray(value) ? value : fallback
  return [...new Set(source.map((item) => String(item ?? '').trim()).filter(Boolean))]
}

export function normalizeSettings(value: Record<string, unknown>): FortuneSettings {
  return {
    trigger_commands: normalizeStringList(value.trigger_commands, ['我的运势']),
    stats_trigger_commands: normalizeStringList(value.stats_trigger_commands, ['运势统计']),
    timezone: String(value.timezone || 'Asia/Shanghai'),
    fortunes: Array.isArray(value.fortunes)
      ? value.fortunes.map((item) => {
          const source = item as Partial<FortuneItem>
          return {
            name: String(source.name ?? ''),
            stars: String(source.stars ?? ''),
            sign: String(source.sign ?? ''),
            explanation: String(source.explanation ?? ''),
          }
        })
      : [],
    special_dates: Array.isArray(value.special_dates)
      ? value.special_dates.map((item) => {
          const source = item as Partial<SpecialDate>
          return { date: String(source.date ?? ''), fortune_name: String(source.fortune_name ?? '') }
        })
      : [],
    good_actions: normalizeStringList(value.good_actions, ['整理计划']),
    bad_actions: normalizeStringList(value.bad_actions, ['熬夜']),
  }
}

export function validateSettings(value: FortuneSettings): string[] {
  const errors: string[] = []
  if (!value.timezone.trim()) errors.push('时区不能为空。')
  if (value.trigger_commands.length === 0) errors.push('至少需要一个运势触发词。')
  if (value.stats_trigger_commands.length === 0) errors.push('至少需要一个统计触发词。')
  if (value.fortunes.length === 0) errors.push('至少需要一条运势。')
  value.fortunes.forEach((item, index) => {
    if (!item.name.trim() || !item.sign.trim() || !item.explanation.trim()) {
      errors.push(`第 ${index + 1} 条运势缺少名称、签文或解签。`)
    }
  })
  return errors
}
