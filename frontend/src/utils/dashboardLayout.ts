// Arrangement of the widgets of the Analytics tab in a 12-column grid.

// Widths a widget may take, in columns
export const WIDGET_WIDTHS = [4, 6, 8, 12]

// A widget of the tab: title is a dictionary key, cols is the standard width,
// hint is the dictionary key of the text behind the «i» of the widget.
// The order of the definitions is the standard order of the widgets.
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

// What is saved in the user's settings; {} is the standard arrangement
export interface DashboardLayout {
  widgets?: PlacedWidget[]
  hidden_widgets?: string[]
}

// The width from the grid steps closest to the wanted one
export function nearestWidth(cols: number): number {
  return WIDGET_WIDTHS.reduce((best, w) => (Math.abs(w - cols) < Math.abs(best - cols) ? w : best))
}

// The placed widgets of a saved layout in their order, completed with the
// standard ones the layout does not know yet (added to the tab later): such a
// widget goes right after its neighbour from the standard order. Entries of
// widgets that do not exist are dropped.
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
    // After the closest preceding standard widget that is on the dashboard
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

// The widgets the user removed from the dashboard, in the standard order
export function hiddenWidgets(saved: DashboardLayout | null | undefined, defs: WidgetDef[]): string[] {
  const hidden = Array.isArray(saved?.hidden_widgets) ? saved!.hidden_widgets! : []
  return defs.map(d => d.id).filter(id => hidden.includes(id))
}
