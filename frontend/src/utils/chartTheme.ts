// Цвета графиков берутся из темы, поэтому график в любой теме выглядит как
// остальная страница.

export function cssVar(name: string, fallback = ''): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

// Оформление подсказки Chart.js, общее для всех графиков
export function tooltipTheme() {
  return {
    backgroundColor: cssVar('--surface-3', 'rgba(15, 16, 17, 0.9)'),
    titleColor: cssVar('--text-bright', '#f7f8f8'),
    bodyColor: cssVar('--text-dim', '#d0d6e0'),
    borderColor: cssVar('--hairline', '#23252a'),
    borderWidth: 1,
    cornerRadius: 8,
    padding: 8,
  }
}

// Часы не более чем с одним знаком после запятой: 12, 7.5
export function roundHours(hours: number): number {
  return Math.round(hours * 10) / 10
}
