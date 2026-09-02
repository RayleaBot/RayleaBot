#!/usr/bin/env node

import { readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { extname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(fileURLToPath(new URL('..', import.meta.url)))
const webRoot = join(root, 'web')
const write = process.argv.includes('--write')
const extensions = new Set(['.mjs', '.ts', '.vue'])
const excludedDirectories = new Set(['coverage', 'dist', 'node_modules', 'playwright-report', 'test-results'])
const targets = [join(webRoot, 'src'), join(webRoot, 'tests'), join(webRoot, 'scripts')]
const files = []

for (const target of targets) {
  collectFiles(target)
}

const violations = []
for (const file of files) {
  const source = readFileSync(file, 'utf8')
  const lines = source.split(/\r?\n/)
  let changed = false
  for (let index = 0; index < lines.length; index += 1) {
    const leadingWhitespace = lines[index].match(/^[\t ]*/)?.[0] ?? ''
    if (!leadingWhitespace.includes('\t')) continue
    if (write) {
      lines[index] = leadingWhitespace.replaceAll('\t', '  ') + lines[index].slice(leadingWhitespace.length)
      changed = true
      continue
    }
    violations.push(`${relative(root, file).replaceAll('\\', '/')}:${index + 1}`)
  }
  if (changed) {
    writeFileSync(file, lines.join('\n'), 'utf8')
  }
}

if (write) {
  console.log(`Normalized indentation in ${files.length} Web source and test files.`)
} else if (violations.length > 0) {
  console.error('Leading tab indentation is not allowed:')
  for (const violation of violations) console.error(`- ${violation}`)
  process.exitCode = 1
} else {
  console.log(`Checked ${files.length} Web source and test files; indentation is clean.`)
}

function collectFiles(directory) {
  if (!statSync(directory, { throwIfNoEntry: false })?.isDirectory()) return
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!excludedDirectories.has(entry.name)) collectFiles(join(directory, entry.name))
      continue
    }
    if (extensions.has(extname(entry.name))) files.push(join(directory, entry.name))
  }
}
