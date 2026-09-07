import { readonly, shallowRef } from 'vue'
export type ToastLevel = 'error' | 'info' | 'success' | 'warning'
interface ToastEntry { id: number; level: ToastLevel; content: string }
const entries = shallowRef<ToastEntry[]>([])
let nextId = 0
export const toasts = readonly(entries)
export function dismissToast(id: number) { entries.value = entries.value.filter(entry => entry.id !== id) }
export function publishToast(level: ToastLevel, content: string) {
  // Bound transient feedback during bursts; persistent problems remain in their page state.
  entries.value = [...entries.value.slice(-3), { id: ++nextId, level, content }]
}