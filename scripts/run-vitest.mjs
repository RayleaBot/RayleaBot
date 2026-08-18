#!/usr/bin/env node

import { existsSync } from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'

const webStorageFlag = '--no-experimental-webstorage'
const existingNodeOptions = process.env.NODE_OPTIONS?.trim() ?? ''
const nodeOptions = existingNodeOptions.split(/\s+/).includes(webStorageFlag)
  ? existingNodeOptions
  : [existingNodeOptions, webStorageFlag].filter(Boolean).join(' ')
const vitestCLI = path.resolve(process.cwd(), 'node_modules', 'vitest', 'vitest.mjs')

if (!existsSync(vitestCLI)) {
  console.error(`Vitest is not installed in ${process.cwd()}; run pnpm install first.`)
  process.exit(1)
}

const result = spawnSync(process.execPath, [vitestCLI, ...process.argv.slice(2)], {
  env: {
    ...process.env,
    NODE_OPTIONS: nodeOptions,
  },
  stdio: 'inherit',
  windowsHide: true,
})

if (result.error) {
  throw result.error
}

process.exitCode = result.status ?? 1
