import { reactive } from 'vue'

// Issues that have just changed: their rows and cards flash briefly
// (class "flash-update" in global.css).
const flashing = reactive(new Set<number>())

const FLASH_MS = 800

export function flashIssues(ids: number[]) {
  if (!ids.length) return
  ids.forEach(id => flashing.add(id))
  setTimeout(() => ids.forEach(id => flashing.delete(id)), FLASH_MS)
}

export function isFlashing(id: number): boolean {
  return flashing.has(id)
}
