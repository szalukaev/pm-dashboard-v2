package handlers

// Limits of a saved dashboard layout: enough for every widget of the tab.
const (
	maxDashboardWidgets = 50
	maxWidgetIDLen      = 50
)

// dashboardWidths are the widths a widget may take in the 12-column grid.
var dashboardWidths = []int{4, 6, 8, 12}

// validDashboardLayout checks the layout of the Analytics tab sent by the
// client: {"widgets": [{"id": "...", "cols": 4|6|8|12, "collapsed": bool}],
// "hidden_widgets": ["..."]}. An empty object resets the layout. The ids
// themselves are not checked here — the widgets live in the client, which
// ignores the ids it does not know.
func validDashboardLayout(v interface{}) bool {
	layout, ok := v.(map[string]interface{})
	if !ok {
		return false
	}
	for field := range layout {
		if field != "widgets" && field != "hidden_widgets" {
			return false
		}
	}

	seen := map[string]bool{}
	validID := func(v interface{}) bool {
		id, isString := v.(string)
		if !isString || id == "" || len(id) > maxWidgetIDLen || seen[id] {
			return false
		}
		seen[id] = true
		return true
	}

	if raw, present := layout["widgets"]; present {
		widgets, ok := raw.([]interface{})
		if !ok || len(widgets) > maxDashboardWidgets {
			return false
		}
		for _, w := range widgets {
			widget, ok := w.(map[string]interface{})
			if !ok || !validID(widget["id"]) || !validDashboardWidth(widget["cols"]) {
				return false
			}
			for field, value := range widget {
				switch field {
				case "id", "cols":
				case "collapsed":
					if _, isBool := value.(bool); !isBool {
						return false
					}
				default:
					return false
				}
			}
		}
	}

	if raw, present := layout["hidden_widgets"]; present {
		hidden, ok := raw.([]interface{})
		if !ok || len(hidden) > maxDashboardWidgets {
			return false
		}
		for _, id := range hidden {
			// A widget is either placed or hidden, never both
			if !validID(id) {
				return false
			}
		}
	}
	return true
}

func validDashboardWidth(v interface{}) bool {
	n, isNumber := v.(float64)
	if !isNumber {
		return false
	}
	for _, width := range dashboardWidths {
		if n == float64(width) {
			return true
		}
	}
	return false
}
