// Проект, как его отдаёт список доступных проектов
export interface ProjectNode {
  id: number
  name: string
  parent_id: number | null
}

// Проекты, которые пользователь отслеживает: выбранные им в настройках вместе
// с их подпроектами. Если ничего не выбрано — все доступные, как и везде.
export function trackedProjects(all: ProjectNode[], selected: number[]): { id: number; name: string }[] {
  if (!selected || selected.length === 0) return all.map(p => ({ id: p.id, name: p.name }))

  const parents = new Map<number, number | null>(all.map(p => [p.id, p.parent_id] as [number, number | null]))
  const isTracked = (id: number) => {
    const seen = new Set<number>()
    for (let current: number | null | undefined = id; current != null && !seen.has(current); current = parents.get(current)) {
      if (selected.includes(current)) return true
      seen.add(current)
    }
    return false
  }
  return all.filter(p => isTracked(p.id)).map(p => ({ id: p.id, name: p.name }))
}
