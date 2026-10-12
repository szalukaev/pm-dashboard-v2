package invoice

import (
	"sort"
	"strings"
)

// Warnings of a template: it can be used, but something in it looks wrong.
const (
	// WarnNoItems: the lines of the invoice have nowhere to go — neither
	// {{items_table}} nor a row with item variables.
	WarnNoItems = "no_items"
	// WarnNoTotal: the amount to pay is not in the document.
	WarnNoTotal = "no_total"
	// WarnItemsTableNotAlone: {{items_table}} shares its line with other
	// text, so no table can be put in its place.
	WarnItemsTableNotAlone = "items_table_not_alone"
	// WarnItemsOutsideTable: item variables stand outside a table row, so
	// only the first line of the invoice would be shown.
	WarnItemsOutsideTable = "items_outside_table"
)

// Report is what a look at a template finds.
type Report struct {
	// Found: the known variables the template uses.
	Found []string `json:"found"`
	// Unknown: what is written as a variable but is not one — a typo, most
	// likely. It would stay in the invoice as it is.
	Unknown []string `json:"unknown"`
	// Warnings: see the Warn* constants.
	Warnings []string `json:"warnings"`
}

// Inspect checks that the file can serve as a template and reports what
// variables it holds. An error means the file is refused.
func Inspect(template []byte) (*Report, error) {
	archive, err := openTemplate(template)
	if err != nil {
		return nil, err
	}

	found, unknown := map[string]bool{}, map[string]bool{}
	tableAlone, tableNotAlone := false, false
	itemsInRow, itemsOutside := false, false

	for _, file := range archive.File {
		if !textPart(file.Name) {
			continue
		}
		raw, err := readPart(file)
		if err != nil {
			return nil, err
		}
		content := joinSplitVariables(string(raw))

		// Where the item variables stand: in a table row or not
		rows := elementSpans(content, "w:tr")
		inRow := func(at int) bool {
			for _, r := range rows {
				if at >= r.start && at < r.end {
					return true
				}
			}
			return false
		}

		for _, p := range leafSpans(elementSpans(content, "w:p")) {
			paragraph := content[p.start:p.end]
			text := paragraphText(paragraph)
			for _, m := range anyBracesRe.FindAllStringSubmatch(text, -1) {
				name := strings.ToLower(strings.TrimSpace(m[1]))
				if !knownVariable(name) {
					unknown["{{"+strings.TrimSpace(m[1])+"}}"] = true
					continue
				}
				found[name] = true
				switch {
				case name == ItemsTable && isItemsTablePlaceholder(paragraph):
					tableAlone = true
				case name == ItemsTable:
					tableNotAlone = true
				case isItemVariable(name) && inRow(p.start):
					itemsInRow = true
				case isItemVariable(name):
					itemsOutside = true
				}
			}
		}
	}

	report := &Report{Found: []string{}, Unknown: []string{}, Warnings: []string{}}
	// In the order of the list of variables, not of the alphabet
	for _, v := range variableOrder {
		if found[v.name] {
			report.Found = append(report.Found, v.name)
		}
	}
	for name := range unknown {
		report.Unknown = append(report.Unknown, name)
	}
	sort.Strings(report.Unknown)

	if !tableAlone && !itemsInRow {
		report.Warnings = append(report.Warnings, WarnNoItems)
	}
	// The built table ends with the amount to pay
	if !found["total"] && !found["total_words"] && !tableAlone {
		report.Warnings = append(report.Warnings, WarnNoTotal)
	}
	if tableNotAlone {
		report.Warnings = append(report.Warnings, WarnItemsTableNotAlone)
	}
	if itemsOutside {
		report.Warnings = append(report.Warnings, WarnItemsOutsideTable)
	}
	return report, nil
}
