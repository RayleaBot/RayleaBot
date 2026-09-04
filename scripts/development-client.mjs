export async function developmentRequest(baseURL, token, route, body, { fetchImpl = fetch, timeoutMs = 55_000 } = {}) {
  const response = await fetchImpl(new URL(route, `${baseURL.replace(/\/$/, '')}/`), {
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'X-Raylea-Launcher-Control': token, ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(timeoutMs),
    redirect: 'error',
  })
  if (!response.ok) throw new Error(`Development request rejected: HTTP ${response.status}`)
  return response.json()
}

export async function synchronizeDevelopmentPlugin({ baseURL, token, artifact, source, request = developmentRequest, sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms)), timeoutMs = 15 * 60_000 }) {
  const result = await request(baseURL, token, 'api/development/plugins/sync', { artifact, source })
  if (result.changed === false && result.task_id === '') return false
  if (result.changed !== true || !result.task_id) throw new Error('Invalid development sync response')
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const task = await request(baseURL, token, `api/development/plugins/sync/${encodeURIComponent(result.task_id)}`)
    if (task.status === 'succeeded') return true
    if (!['pending', 'running'].includes(task.status)) throw new Error(`Development sync failed: ${task.error_code || task.status}`)
    await sleep(100)
  }
  throw new Error('Development sync timed out; inspect the installation task before retrying')
}
