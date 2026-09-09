import { mkdir, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'

const ianaVersion = '2026c'
const cldrVersion = '48.0.0'
const ianaRoot = `https://data.iana.org/time-zones/tzdb-${ianaVersion}`
const cldrRoot = `https://raw.githubusercontent.com/unicode-org/cldr-json/${cldrVersion}`

async function download(url) {
  const response = await fetch(url)
  if (!response.ok) throw new Error(`${url}: HTTP ${response.status}`)
  return response.text()
}

const windowsSource = `${cldrRoot}/cldr-json/cldr-core/supplemental/windowsZones.json`
const [tzdata, territories, namesText, windowsText, license] = await Promise.all([
  download(`${ianaRoot}/tzdata.zi`),
  download(`${ianaRoot}/zone.tab`),
  download(`${cldrRoot}/cldr-json/cldr-dates-full/main/zh/timeZoneNames.json`),
  download(windowsSource),
  download(`${cldrRoot}/LICENSE`),
])
const names = JSON.parse(namesText).main.zh.dates.timeZoneNames.zone
const links = new Map()
const identifiers = new Set()
for (const line of tzdata.split('\n')) {
  const fields = line.trim().split(/\s+/)
  if (fields[0] === 'Z') identifiers.add(fields[1])
  if (fields[0] === 'L') {
    identifiers.add(fields[2])
    links.set(fields[2], fields[1])
  }
}
const countries = new Map()
for (const line of territories.split('\n')) {
  if (!line || line.startsWith('#')) continue
  const fields = line.split('\t')
  countries.set(fields[2], fields[0])
}
function canonical(id) {
  const visited = new Set()
  while (links.has(id) && !visited.has(id)) { visited.add(id); id = links.get(id) }
  return id
}
function cityName(id) {
  return id.split('/').reduce((node, segment) => node?.[segment], names)?.exemplarCity
}
const canonicalCities = new Map()
for (const id of identifiers) {
  const city = cityName(id)
  if (city && !canonicalCities.has(canonical(id))) canonicalCities.set(canonical(id), city)
}
const regions = new Intl.DisplayNames('zh-CN', { type: 'region' })
const zones = [...identifiers].filter(id => id !== 'Factory').sort().map(id => {
  const canonicalId = canonical(id)
  const country = countries.get(id) || countries.get(canonicalId)
  return {
    id,
    city: id === 'UTC' || id === 'Etc/UTC' ? '协调世界时' : id.startsWith('Etc/GMT') ? '固定偏移' : cityName(id) || cityName(canonicalId) || canonicalCities.get(canonicalId) || id.split('/').at(-1).replaceAll('_', ' '),
    region: country ? regions.of(country) : '',
    ...(id !== canonicalId ? { canonical: canonicalId } : {}),
  }
})
if (zones.length < 500 || !zones.some(zone => zone.id === 'Asia/Shanghai')) throw new Error('Incomplete IANA timezone catalog')
const windowsZones = JSON.parse(windowsText).supplemental.windowsZones.mapTimezones
  .map(entry => entry.mapZone).filter(zone => zone._territory === '001')
  .map(zone => {
    // Modernize legacy spellings while retaining named regional representatives.
    // Canonicalizing every link would turn Reykjavik into Abidjan, for example.
    const mapped = zone._type
    const id = mapped === 'Etc/UTC' ? 'UTC' : countries.has(mapped) ? mapped : canonical(mapped)
    if (!identifiers.has(id)) throw new Error(`Missing Windows timezone representative: ${mapped}`)
    return { windowsId: zone._other, id }
  })
if (windowsZones.length < 130) throw new Error('Incomplete Windows timezone mapping')
const output = { ianaVersion, cldrVersion, sources: [`${ianaRoot}/tzdata.zi`, `${ianaRoot}/zone.tab`, `${cldrRoot}/cldr-json/cldr-dates-full/main/zh/timeZoneNames.json`, windowsSource], windowsZones, zones }
await writeFile(fileURLToPath(new URL('../web/src/lib/time-zones.generated.json', import.meta.url)), `${JSON.stringify(output, null, 2)}\n`)
await mkdir(fileURLToPath(new URL('../web/public/licenses/', import.meta.url)), { recursive: true })
await writeFile(fileURLToPath(new URL('../web/public/licenses/time-zones.txt', import.meta.url)), `IANA timezone identifiers and territory data (${ianaVersion}) are in the public domain.\nCity names and Windows timezone mappings derive from Unicode CLDR ${cldrVersion}.\n\n${license}`)
console.log(`Generated ${zones.length} timezone identifiers and ${windowsZones.length} Windows representatives from IANA ${ianaVersion} and CLDR ${cldrVersion}.`)
