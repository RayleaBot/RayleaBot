import type { StaticApiRoute } from '@/lib/api-path'
import { getCurrentScope, onScopeDispose, ref } from 'vue'
import { getDisplayErrorMessage } from '@/lib/error-text'

export type CollectionQuery = Record<string, string | undefined>
type Page = { total: number; next_cursor?: string }

export function collectionURL<Path extends StaticApiRoute>(path: Path, query: CollectionQuery, cursor = ''): Path | `${Path}?${string}` {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) if (value?.trim()) params.set(key, value.trim())
  if (cursor) params.set('cursor', cursor)
  return params.size ? `${path}?${params}` : path
}

// Refresh only the pages the user already requested. A new search starts at the
// first page; it never walks all cursors to emulate an unbounded collection.
export function createCollectionPager<Response extends Page>(options: {
  request: (query: CollectionQuery, cursor: string, signal: AbortSignal) => Promise<Response>
  apply: (response: Response, append: boolean, query: Readonly<CollectionQuery>) => void
}) {
  const total = ref(0)
  const nextCursor = ref('')
  const loading = ref(false)
  const loadingMore = ref(false)
  const error = ref<string | null>(null)
  const loaded = ref(false)
  let currentQuery: CollectionQuery = {}
  let pages = 1
  let generation = 0
  let controller: AbortController | null = null
  let pending: Promise<Response | undefined> | null = null

  function cancel() { generation++; controller?.abort(); controller = null; loading.value = false; loadingMore.value = false }
  if (getCurrentScope()) onScopeDispose(cancel)

  function load(query?: CollectionQuery, signal?: AbortSignal, append = false): Promise<Response | undefined> {
    if (append && (!nextCursor.value || loading.value || loadingMore.value)) return Promise.resolve(undefined)
    controller?.abort()
    const active = new AbortController()
    controller = active
    const request = ++generation
    const requestSignal = signal ? AbortSignal.any([signal, active.signal]) : active.signal
    const pageCount = query === undefined && !append ? pages : 1
    if (query !== undefined) currentQuery = { ...query }
    const selectedQuery = { ...currentQuery }
    const startCursor = append ? nextCursor.value : ''
    if (append) loadingMore.value = true
    else { loading.value = true; loadingMore.value = false }
    error.value = null
    const operation = (async () => {
      let cursor = startCursor
      let response: Response | undefined
      try {
        for (let page = 0; page < pageCount; page++) {
          response = await options.request(selectedQuery, cursor, requestSignal)
          requestSignal.throwIfAborted()
          if (request !== generation) return undefined
          options.apply(response, append || page > 0, selectedQuery)
          total.value = response.total
          nextCursor.value = response.next_cursor ?? ''
          loaded.value = true
          pages = append ? pages + 1 : page + 1
          cursor = nextCursor.value
          if (!cursor) break
        }
        return response
      } catch (cause) {
        if (request === generation && !requestSignal.aborted) error.value = getDisplayErrorMessage(cause, 'errors.common.loadFailed')
        throw cause
      } finally {
        if (request === generation) { loading.value = false; loadingMore.value = false; controller = null }
      }
    })()
    pending = operation
    void operation.finally(() => { if (pending === operation) pending = null }).catch(() => undefined)
    return operation
  }

  function ensure(signal?: AbortSignal) { return pending ?? (loaded.value ? Promise.resolve(undefined) : load(undefined, signal)) }
  function loadMore(signal?: AbortSignal) { return load(undefined, signal, true) }
  return { total, nextCursor, loading, loadingMore, error, loaded, load, loadMore, ensure, cancel }
}

export function mergeCollectionItems<T>(previous: T[], incoming: T[], key: (item: T) => string) {
  return [...new Map([...previous, ...incoming].map(item => [key(item), item])).values()]
}
