import assert from 'node:assert/strict'
import test from 'node:test'
import { developmentRequest, synchronizeDevelopmentPlugin } from '../development-client.mjs'

test('development control refuses redirects and sends only its scoped token', async () => {
  await developmentRequest('http://127.0.0.1:1234', 'fixture-only-token', 'api/development/status', undefined, { fetchImpl: async (_url, options) => {
    assert.equal(options.redirect, 'error')
    assert.deepEqual(options.headers, { 'X-Raylea-Launcher-Control': 'fixture-only-token' })
    return { ok: true, json: async () => ({}) }
  } })
})

test('sync distinguishes no-op, completed installation and rejected candidate', async () => {
  for (const [result, terminal, expected] of [
    [{ changed: false, task_id: '' }, '', false],
    [{ changed: true, task_id: 'fixture-task' }, 'succeeded', true],
    [{ changed: true, task_id: 'fixture-task' }, 'failed', 'plugin.install_failed'],
  ]) {
    let polls = 0
    const operation = synchronizeDevelopmentPlugin({
      baseURL: 'http://127.0.0.1:1234', token: 'fixture-only-token', artifact: '/fixture/artifact', source: '/fixture/source', sleep: async () => {},
      request: async (_base, _token, _route, body) => body ? result : (++polls === 1 ? { status: 'running' } : { status: terminal, error_code: 'plugin.install_failed' }),
    })
    if (typeof expected === 'string') await assert.rejects(operation, /plugin.install_failed/)
    else assert.equal(await operation, expected)
    if (expected === false) assert.equal(polls, 0)
  }
})
