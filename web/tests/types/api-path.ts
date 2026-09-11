import { apiDownload, apiRequest } from '@/lib/http'
import { collectionURL } from '@/lib/collection-pager'
import { apiPath, type ApiPath } from '@/lib/api-path'

declare const pluginId: string
declare const arbitraryPath: string
declare const action: 'enable' | 'disable' | 'reload'
const dynamic: ApiPath = apiPath('/api/plugins/{plugin_id}/settings', { plugin_id: pluginId })
void apiRequest('/api/adapters')
void apiRequest(dynamic)
void apiRequest(apiPath(`/api/plugins/{plugin_id}/${action}`, { plugin_id: pluginId }))
void apiRequest(apiPath('/api/plugin-store/plugins/{plugin_id}', { plugin_id: pluginId }, new URLSearchParams({ source_id: 'official' })))
void apiRequest(collectionURL('/api/plugins', { query: 'weather' }, 'cursor'))
void apiDownload('/api/system/diagnostics/export')
// @ts-expect-error The contract has no such static operation.
void apiRequest('/api/unknown-resource')
// @ts-expect-error A dynamic ID cannot absorb undeclared suffixes.
void apiRequest('/api/plugins/foo/nonexistent')
// @ts-expect-error Adapter IDs cannot absorb undeclared suffixes either.
void apiRequest('/api/adapters/x/bogus')
// @ts-expect-error Raw dynamic URL strings must go through the template builder.
void apiRequest(`/api/plugins/${pluginId}/settings`)
// @ts-expect-error Arbitrary strings bypass route checking.
void apiRequest(arbitraryPath)
// @ts-expect-error A misspelled template is not an OpenAPI route.
apiPath('/api/plugins/{plugin_id}/nonexistent', { plugin_id: pluginId })
// @ts-expect-error Path parameter names must match this exact route.
apiPath('/api/plugins/{plugin_id}/settings', { id: pluginId })
// @ts-expect-error Missing path parameters cannot produce a URL.
apiPath('/api/plugins/{plugin_id}/settings', {})
// @ts-expect-error A second parameter is required for this route.
apiPath('/api/governance/blacklist/entries/{entry_type}/{target_id}', { entry_type: 'user' })
// @ts-expect-error Collection endpoints must exist in the OpenAPI paths map.
collectionURL('/api/unknown-collection', {})
// @ts-expect-error Collection pagination cannot send an unexpanded route template.
collectionURL('/api/plugins/{plugin_id}/settings', {})
