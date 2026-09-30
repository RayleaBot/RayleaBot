import { onBeforeUnmount, onMounted } from 'vue'

import { useUiShellStore } from '@/stores/ui-shell'

// Ctrl/Cmd+K opens the page search and Alt+Shift+S opens the preferences, also from inside fields.
export function useShellShortcuts() {
  const uiShellStore = useUiShellStore()

  function handleShortcut(event: KeyboardEvent) {
    const key = event.key.toLowerCase()
    if ((event.metaKey || event.ctrlKey) && key === 'k') {
      event.preventDefault()
      uiShellStore.openSearch()
    } else if (event.altKey && event.shiftKey && key === 's') {
      event.preventDefault()
      uiShellStore.openSettings()
    }
  }

  onMounted(() => document.addEventListener('keydown', handleShortcut))
  onBeforeUnmount(() => document.removeEventListener('keydown', handleShortcut))
}
