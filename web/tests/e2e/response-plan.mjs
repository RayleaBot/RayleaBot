/** Fixtures deliberately do not interpret request payloads or infer business state. */
export class ResponsePlan {
  #responses = new Map()

  clear() { this.#responses.clear() }

  set(responses) {
    for (const response of responses) {
      const key = `${response.method ?? 'GET'} ${response.path}`
      this.#responses.set(key, structuredClone(response))
    }
  }

  take(method, path) {
    const response = this.#responses.get(`${method} ${path}`)
    if (!response) return null
    if (response.after) this.set(response.after)
    return structuredClone(response)
  }
}
