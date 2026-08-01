export interface ConfigPanelSettings { default_city: string; unit: 'celsius' | 'fahrenheit' }

export function normalizeSettings(value: Record<string, unknown>): ConfigPanelSettings {
  return {
    default_city: typeof value.default_city === 'string' ? value.default_city : '北京',
    unit: value.unit === 'fahrenheit' ? 'fahrenheit' : 'celsius',
  }
}
