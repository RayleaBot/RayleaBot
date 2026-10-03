import { readFileSync } from 'node:fs'
import YAML from 'yaml'
import { describe, expect, it } from 'vitest'
import { i18n } from '@/i18n'
import { errorCatalog } from '@/types/error-codes.generated'

describe('dynamic locale coverage', () => {
  it('has a message for every generated formal error code', () => {
    for (const code of Object.keys(errorCatalog)) expect(i18n.global.te(`errors.${code}`), code).toBe(true)
  })

  it('covers the contract enums used as dynamic status labels', () => {
    const contract = YAML.parse(readFileSync('../contracts/web-api.openapi.yaml', 'utf8'))
    for (const [schema, prefix] of [
      ['PluginState', 'display.pluginStates'],
      ['AdapterState', 'display.adapterStates'],
    ]) {
      const values = contract.components.schemas[schema].enum
      expect(values.length).toBeGreaterThan(0)
      for (const value of values) expect(i18n.global.te(`${prefix}.${value}`), `${prefix}.${value}`).toBe(true)
    }
    const diagnosisKinds = contract.components.schemas.PluginStateDiagnosis.properties.kind.enum
    expect(diagnosisKinds.length).toBeGreaterThan(0)
    for (const kind of diagnosisKinds) expect(i18n.global.te(`plugins.overview.health.diagnoses.${kind}.label`), kind).toBe(true)
  })

  it('labels every event type the host emits', () => {
    const protocol = JSON.parse(readFileSync('../contracts/plugin-protocol.schema.json', 'utf8'))
    const eventTypes: string[] = protocol.$defs.event.allOf[1].properties.event.properties.event_type.enum
    expect(eventTypes.length).toBeGreaterThan(0)
    for (const eventType of eventTypes) expect(i18n.global.te(`display.eventTypes.${eventType}`), eventType).toBe(true)
  })
})
