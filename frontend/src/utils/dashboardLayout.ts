// Раскладка виджетов вкладки «Аналитика» в сетке из 12 колонок.

// Ширины, которые может занимать виджет, в колонках
export const WIDGET_WIDTHS = [4, 6, 8, 12]

// Виджет вкладки: title — ключ словаря, cols — стандартная ширина, hint —
// ключ словаря для текста за значком «i» виджета. Порядок определений —
// стандартный порядок виджетов.
export interface WidgetDef {
  id: string
  title: string
  hint?: string
  cols: number
}

export interface PlacedWidget {
  id: string
  cols: number
  collapsed?: boolean
}

// То, что сохраняется в настройках пользователя; {} — стандартная раскладка
export interface DashboardLayout {
  widgets?: PlacedWidget[]
  hidden_widgets?: string[]
}

// Ширина из шагов сетки, ближайшая к желаемой
export function nearestWidth(cols: number): number {
  return WIDGET_WIDTHS.reduce((best, w) => (Math.abs(w - cols) < Math.abs(best - cols) ? w : best))
}

// Размещённые виджеты сохранённой раскладки в их порядке, дополненные
// стандартными, которых раскладка ещё не знает (добавлены на вкладку позже):
// такой виджет встаёт сразу после своего соседа из стандартного порядка.
// Записи несуществующих виджетов отбрасываются.
export function placedWidgets(saved: DashboardLayout | null | undefined, defs: WidgetDef[]): PlacedWidget[] {
  const known = new Set(defs.map(d => d.id))
  const hidden = new Set(hiddenWidgets(saved, defs))
  const placed: PlacedWidget[] = []
  for (const w of Array.isArray(saved?.widgets) ? saved!.widgets! : []) {
    if (!w || !known.has(w.id) || hidden.has(w.id) || placed.some(p => p.id === w.id)) continue
    placed.push({ id: w.id, cols: nearestWidth(Number(w.cols) || 12), collapsed: w.collapsed === true })
  }

  defs.forEach((def, index) => {
    if (hidden.has(def.id) || placed.some(p => p.id === def.id)) return
    // После ближайшего предыдущего стандартного виджета, который есть на дашборде
    let at = 0
    for (let i = index - 1; i >= 0; i--) {
      const found = placed.findIndex(p => p.id === defs[i].id)
      if (found !== -1) {
        at = found + 1
        break
      }
    }
    placed.splice(at, 0, { id: def.id, cols: def.cols, collapsed: false })
  })
  return placed
}

// Виджеты, которые пользователь убрал с дашборда, в стандартном порядке
export function hiddenWidgets(saved: DashboardLayout | null | undefined, defs: WidgetDef[]): string[] {
  const hidden = Array.isArray(saved?.hidden_widgets) ? saved!.hidden_widgets! : []
  return defs.map(d => d.id).filter(id => hidden.includes(id))
}
