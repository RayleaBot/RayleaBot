import catalog from './time-zones.generated.json'
import { DEFAULT_TIME_ZONE } from './time-zone'

// Keep familiar cities alongside every Windows representative. Equal offsets
// today do not imply equal regional DST rules or future civil-time changes.
const additionalCityIDs = [
  'UTC',
  'Pacific/Honolulu', 'America/Anchorage',
  'America/Vancouver', 'America/Los_Angeles', 'America/Tijuana', 'America/Phoenix',
  'America/Denver', 'America/Edmonton', 'America/Regina', 'America/Winnipeg',
  'America/Chicago', 'America/Mexico_City', 'America/Guatemala', 'America/Bogota',
  'America/Lima', 'America/Toronto', 'America/New_York', 'America/Havana',
  'America/Halifax', 'America/Caracas', 'America/La_Paz', 'America/Santiago',
  'America/St_Johns', 'America/Sao_Paulo', 'America/Argentina/Buenos_Aires', 'America/Montevideo',
  'Europe/London', 'Europe/Lisbon', 'Europe/Paris', 'Europe/Berlin', 'Europe/Madrid',
  'Europe/Rome', 'Europe/Amsterdam', 'Europe/Brussels', 'Europe/Zurich', 'Europe/Warsaw',
  'Europe/Prague', 'Europe/Stockholm', 'Europe/Helsinki', 'Europe/Athens',
  'Europe/Bucharest', 'Europe/Kyiv', 'Europe/Istanbul', 'Europe/Moscow',
  'Africa/Casablanca', 'Africa/Abidjan', 'Africa/Lagos', 'Africa/Algiers',
  'Africa/Cairo', 'Africa/Johannesburg', 'Africa/Nairobi',
  'Asia/Jerusalem', 'Asia/Beirut', 'Asia/Riyadh', 'Asia/Baghdad', 'Asia/Dubai',
  'Asia/Tehran', 'Asia/Baku', 'Asia/Tbilisi', 'Asia/Karachi', 'Asia/Tashkent',
  'Asia/Kolkata', 'Asia/Colombo', 'Asia/Kathmandu', 'Asia/Dhaka', 'Asia/Yangon',
  'Asia/Bangkok', 'Asia/Ho_Chi_Minh', 'Asia/Jakarta', 'Asia/Shanghai', 'Asia/Hong_Kong',
  'Asia/Macau', 'Asia/Taipei', 'Asia/Singapore', 'Asia/Kuala_Lumpur', 'Asia/Manila',
  'Asia/Tokyo', 'Asia/Seoul', 'Asia/Ulaanbaatar', 'Asia/Yekaterinburg',
  'Asia/Novosibirsk', 'Asia/Irkutsk', 'Asia/Vladivostok',
  'Australia/Perth', 'Australia/Darwin', 'Australia/Adelaide', 'Australia/Brisbane',
  'Australia/Sydney', 'Australia/Melbourne', 'Pacific/Guam', 'Pacific/Auckland', 'Pacific/Fiji',
] as const

export const PICKER_TIME_ZONE_IDS = [...new Set([
  ...catalog.windowsZones.map(zone => zone.id), ...additionalCityIDs,
])]

const byID = new Map(catalog.zones.map(zone => [zone.id, zone]))
const pickerZones = PICKER_TIME_ZONE_IDS.map(id => {
  const zone = byID.get(id)
  if (!zone) throw new Error(`Missing representative timezone: ${id}`)
  return { ...zone, retained: false }
})

export function getTimeZonePickerChoices(currentID = DEFAULT_TIME_ZONE) {
  const id = currentID.trim() || DEFAULT_TIME_ZONE
  if (pickerZones.some(zone => zone.id === id)) return pickerZones
  const selected = byID.get(id) ?? { id, city: id.split('/').at(-1)!.replaceAll('_', ' '), region: '' }
  return [...pickerZones, { ...selected, retained: true }]
}
