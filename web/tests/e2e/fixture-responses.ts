import { expect, type APIRequestContext } from '@playwright/test'

type FixtureResponse = {
  method?: string
  path: string
  status?: number
  body?: unknown
  after?: Array<Omit<FixtureResponse, 'after'>>
}

/** A scripted response and optional later reads; no request-body interpretation. */
export async function respondWith(request: APIRequestContext, responses: FixtureResponse[]) {
  const result = await request.post('http://127.0.0.1:4010/__test/responses', { data: { responses } })
  expect(result.ok()).toBe(true)
}
