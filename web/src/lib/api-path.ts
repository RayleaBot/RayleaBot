import type { paths } from '@/types/generated'

type ApiRoute = keyof paths
type ParameterizedApiRoute = Extract<ApiRoute, `${string}{${string}}${string}`>
export type StaticApiRoute = Exclude<ApiRoute, ParameterizedApiRoute>

type ParameterNames<Route extends string> = Route extends `${string}{${infer Name}}${infer Rest}`
  ? Name | ParameterNames<Rest> : never
type Parameters<Route extends ParameterizedApiRoute> = Record<ParameterNames<Route>, string>

declare const encodedApiPath: unique symbol
// Dynamic URLs can only be created from a declared template and encoded values.
// A raw `${string}` parameter would also match misspelled trailing route segments.
type EncodedApiPath = string & { readonly [encodedApiPath]: true }
export type ApiPath = StaticApiRoute | `${StaticApiRoute}?${string}` | EncodedApiPath

export function apiPath<Route extends ParameterizedApiRoute>(
  template: Route,
  parameters: Parameters<NoInfer<Route>>,
  query?: URLSearchParams,
): EncodedApiPath {
  const pathname = template.replace(/\{([^}]+)\}/g, (_, name: ParameterNames<Route>) => {
    const value = parameters[name]
    // Browsers normalize standalone dot segments even when percent-encoded.
    if (typeof value !== 'string' || !value || value === '.' || value === '..') {
      throw new TypeError(`Invalid API path parameter: ${name}`)
    }
    return encodeURIComponent(value)
  })
  return (query?.size ? `${pathname}?${query}` : pathname) as EncodedApiPath
}
