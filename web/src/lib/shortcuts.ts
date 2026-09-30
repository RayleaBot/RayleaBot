// Shortcut labels follow the platform the page runs on, so the search pill and the preferences list agree.
const applePlatform = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)

export const searchShortcutLabel = applePlatform ? '⌘ K' : 'Ctrl K'
export const settingsShortcutLabel = applePlatform ? '⌥ ⇧ S' : 'Alt Shift S'
