// Аватар человека без фото: инициалы на цветном круге.

export function initials(name?: string | null): string {
  if (!name) return '—'
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map(p => p[0]).join('').toUpperCase()
}

// Цвет аватара постоянен для одного и того же имени
const AVATAR_COLORS = ['#5e81ac', '#b48ead', '#8faa70', '#d08770', '#88c0d0', '#a3be8c', '#bf616a', '#ebcb8b']
export function avatarColor(name?: string | null): string {
  if (!name) return 'var(--text-faint)'
  let hash = 0
  for (const ch of name) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return AVATAR_COLORS[hash % AVATAR_COLORS.length]
}
