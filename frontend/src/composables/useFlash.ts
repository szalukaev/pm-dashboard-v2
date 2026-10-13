import { reactive } from 'vue'

// Только что изменившиеся задачи: их строки и карточки ненадолго
// подсвечиваются (класс "flash-update" в global.css).
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
