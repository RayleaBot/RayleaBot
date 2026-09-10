// Coalesce bursts and preserve one final refresh when a change arrives in flight.
export function createRefreshScheduler(refresh: (signal: AbortSignal) => Promise<unknown>, delayMs = 120) {
  let timer: ReturnType<typeof setTimeout> | undefined
  let active: AbortController | undefined
  let queued = false

  async function run() {
    timer = undefined
    const controller = new AbortController()
    active = controller
    try {
      await refresh(controller.signal)
    } catch {
      // The owning store retains its last usable snapshot and exposes failures.
    } finally {
      if (active === controller) {
        active = undefined
        if (queued) {
          queued = false
          schedule()
        }
      }
    }
  }

  function schedule() {
    if (active) {
      queued = true
      return
    }
    if (timer === undefined) timer = setTimeout(() => { void run() }, delayMs)
  }

  function cancel() {
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
    queued = false
    active?.abort()
    active = undefined
  }

  return { schedule, cancel }
}
