export interface LogDetailWindowPosition {
  left: number
  top: number
}

const positionMemory = new Map<string, LogDetailWindowPosition>()

export function readLogDetailWindowPosition(memoryKey: string) {
  if (!memoryKey) {
    return null
  }

  const stored = positionMemory.get(memoryKey)
  return stored ? { ...stored } : null
}

export function writeLogDetailWindowPosition(memoryKey: string, position: LogDetailWindowPosition) {
  if (!memoryKey) {
    return
  }

  positionMemory.set(memoryKey, { ...position })
}

export function clearLogDetailWindowPositionMemory() {
  positionMemory.clear()
}
