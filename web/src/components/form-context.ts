import { inject, type ComputedRef, type InjectionKey } from 'vue'

export interface FieldContext {
  id: string
  descriptionId: string
  error?: string
  required?: boolean
}
export const fieldContextKey: InjectionKey<ComputedRef<FieldContext>> = Symbol('app-field')
export function useFieldContext() { return inject(fieldContextKey, undefined) }
