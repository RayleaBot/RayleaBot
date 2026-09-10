import { readFileSync } from 'node:fs'
import YAML from 'yaml'
import { describe, expect, it } from 'vitest'
import { i18n } from '@/i18n'
import { errorCatalog } from '@/types/error-codes.generated'

describe('dynamic locale coverage', () => {
  it('has a message for every generated formal error code', () => {
    for (const value of Object.values(errorCatalog)) expect(i18n.global.te(value.messageKey), value.messageKey).toBe(true)
  })

  it('covers the contract enums used as dynamic status labels', () => {
    const contract = YAML.parse(readFileSync('../contracts/web-api.openapi.yaml', 'utf8'))
    for (const [schema, prefix] of [
      ['PluginState', 'display.pluginStates'],
      ['AdapterState', 'display.adapterStates'],
      ['ProtocolReadinessStatus', 'display.readinessStatuses'],
    ]) {
      const values = contract.components.schemas[schema].enum
      expect(values.length).toBeGreaterThan(0)
      for (const value of values) expect(i18n.global.te(`${prefix}.${value}`), `${prefix}.${value}`).toBe(true)
    }
  })
})
