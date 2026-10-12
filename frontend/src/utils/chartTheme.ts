// Colours of the charts come from the theme, so a chart looks like the rest
// of the page in every theme.

export function cssVar(name: string, fallback = ''): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

// The look of a Chart.js tooltip shared by every chart
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

// Hours with at most one decimal: 12, 7.5
export function roundHours(hours: number): number {
  return Math.round(hours * 10) / 10
}
