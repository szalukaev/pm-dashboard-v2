package invoice

import (
	"sort"
	"strings"
)

// Предупреждения по шаблону: пользоваться им можно, но что-то в нём выглядит не так.
const (
	// WarnNoItems: позициям счёта некуда попасть — нет ни {{items_table}},
	// ни строки с переменными позиции.
	WarnNoItems = "no_items"
	// WarnNoTotal: в документе нет суммы к оплате.
	WarnNoTotal = "no_total"
	// WarnItemsTableNotAlone: {{items_table}} стоит в одной строке с другим
	// текстом, поэтому таблицу на её место поставить нельзя.
	WarnItemsTableNotAlone = "items_table_not_alone"
	// WarnItemsOutsideTable: переменные позиции стоят вне строки таблицы,
	// поэтому в счёте оказалась бы только первая позиция.
	WarnItemsOutsideTable = "items_outside_table"
)

// Report — что нашла проверка шаблона.
type Report struct {
	// Found: известные переменные, которые использует шаблон.
	Found []string `json:"found"`
	// Unknown: то, что записано как переменная, но ею не является, — скорее
	// всего, опечатка. В счёте это осталось бы как есть.
	Unknown []string `json:"unknown"`
	// Warnings: см. константы Warn*.
	Warnings []string `json:"warnings"`
}

// Inspect проверяет, что файл годится в шаблоны, и сообщает, какие в нём
// переменные. Ошибка значит, что файл не принят.
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

		// Где стоят переменные позиции: в строке таблицы или нет
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
	// В порядке списка переменных, а не по алфавиту
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
	// Построенная таблица заканчивается суммой к оплате
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
