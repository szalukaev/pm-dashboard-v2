package invoice

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Пределы для шаблона: документ на страницу-две занимает десятки килобайт;
// то, что намного больше, шаблоном счёта не является.
const (
	MaxTemplateSize  = 5 << 20  // файл .docx
	maxUnpackedSize  = 60 << 20 // всё его содержимое в распакованном виде
	maxDocumentParts = 2000     // файлов в архиве
)

// Что может быть не так с файлом, предложенным в качестве шаблона.
var (
	ErrNotDocx  = errors.New("the file is not a .docx document")
	ErrMacros   = errors.New("documents with macros are not accepted")
	ErrTooLarge = errors.New("the document is too large")
)

// Render заполняет шаблон данными счёта и возвращает готовый .docx.
func Render(template []byte, data Data) ([]byte, error) {
	archive, err := openTemplate(template)
	if err != nil {
		return nil, err
	}
	if len(data.Items) == 0 {
		// Счёт без позиций — всё равно счёт за что-то
		data.Items = []Item{{Name: data.ContractName, Quantity: 1}}
	}

	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, file := range archive.File {
		content, err := readPart(file)
		if err != nil {
			return nil, err
		}
		if textPart(file.Name) {
			content = []byte(fillPart(string(content), data))
		}
		w, err := writer.Create(file.Name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(content); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// openTemplate проверяет, что байты — документ Word, пригодный в шаблоны.
func openTemplate(template []byte) (*zip.Reader, error) {
	if len(template) > MaxTemplateSize {
		return nil, ErrTooLarge
	}
	archive, err := zip.NewReader(bytes.NewReader(template), int64(len(template)))
	if err != nil {
		return nil, ErrNotDocx
	}
	if len(archive.File) > maxDocumentParts {
		return nil, ErrTooLarge
	}
	hasDocument := false
	var unpacked uint64
	for _, file := range archive.File {
		unpacked += file.UncompressedSize64
		name := strings.ToLower(file.Name)
		if strings.HasSuffix(name, "vbaproject.bin") {
			return nil, ErrMacros
		}
		if file.Name == "word/document.xml" {
			hasDocument = true
		}
	}
	if !hasDocument {
		return nil, ErrNotDocx
	}
	if unpacked > maxUnpackedSize {
		return nil, ErrTooLarge
	}
	return archive, nil
}

func readPart(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, ErrNotDocx
	}
	defer rc.Close()
	// Размер в заголовке может быть ложным: больше предела не читается
	content, err := io.ReadAll(io.LimitReader(rc, maxUnpackedSize+1))
	if err != nil {
		return nil, ErrNotDocx
	}
	if len(content) > maxUnpackedSize {
		return nil, ErrTooLarge
	}
	return content, nil
}

var textPartName = regexp.MustCompile(`^word/(document|header\d*|footer\d*|footnotes|endnotes)\.xml$`)

// textPart сообщает, содержит ли файл документа текст, в котором может
// стоять переменная: основной текст, колонтитулы, сноски.
func textPart(name string) bool {
	return textPartName.MatchString(name)
}

// fillPart подставляет переменные в одной XML-части документа.
func fillPart(content string, data Data) string {
	content = joinSplitVariables(content)
	content = insertItemsTable(content, data)
	content = repeatItemRows(content, data.Items)
	return substitute(content, data.Values())
}

// ─── Поиск элементов ───

// span — элемент XML-текста: [start, end) покрывает его вместе с тегами.
type span struct {
	start, end int
	// nested: внутри есть элемент того же вида.
	nested bool
}

// elementSpans находит в тексте все <tag …>…</tag>, как бы они ни были
// вложены. У самозакрывающихся элементов содержимого нет, они пропускаются.
func elementSpans(content, tag string) []span {
	// \b не даёт "w:p" совпасть с "w:pPr", а "w:tr" — с "w:trPr"
	re := regexp.MustCompile(`<(/?)` + regexp.QuoteMeta(tag) + `\b[^>]*?(/?)>`)
	var spans []span
	var open []int // номера в spans у элементов, которые ещё не закрыты
	for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
		closing := m[2] != m[3]
		selfClosing := m[4] != m[5]
		switch {
		case selfClosing:
		case !closing:
			spans = append(spans, span{start: m[0], end: -1})
			open = append(open, len(spans)-1)
		case len(open) > 0:
			last := open[len(open)-1]
			open = open[:len(open)-1]
			spans[last].end = m[1]
			if len(open) > 0 {
				spans[open[len(open)-1]].nested = true
			}
		}
	}
	// То, что так и не закрылось, элементом не считается
	closed := spans[:0]
	for _, s := range spans {
		if s.end > 0 {
			closed = append(closed, s)
		}
	}
	return closed
}

// rewrite заменяет заданные участки текста тем, что возвращает для каждого
// replace. Участки не должны пересекаться.
func rewrite(content string, spans []span, replace func(element string) string) string {
	if len(spans) == 0 {
		return content
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var b strings.Builder
	last := 0
	for _, s := range spans {
		b.WriteString(content[last:s.start])
		b.WriteString(replace(content[s.start:s.end]))
		last = s.end
	}
	b.WriteString(content[last:])
	return b.String()
}

// leafSpans оставляет элементы, внутри которых нет элемента того же вида.
func leafSpans(spans []span) []span {
	var leaves []span
	for _, s := range spans {
		if !s.nested {
			leaves = append(leaves, s)
		}
	}
	return leaves
}

// ─── Переменные, разбитые Word ───

var (
	textRun     = regexp.MustCompile(`<w:t\b([^>]*)>([^<]*)</w:t>`)
	variableRe  = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)
	anyBracesRe = regexp.MustCompile(`\{\{([^{}]*)\}\}`)
)

// joinSplitVariables собирает обратно переменные, которые разбил Word.
//
// Word хранит абзац кусками текста («runs») и режет их где угодно: после
// проверки орфографии или смены шрифта "{{total}}" может храниться как
// "{{", "total" и "}}". Такая переменная целиком переносится в кусок, в
// котором начинается; к ней применяется оформление этого куска.
func joinSplitVariables(content string) string {
	return rewrite(content, leafSpans(elementSpans(content, "w:p")), joinInParagraph)
}

func joinInParagraph(paragraph string) string {
	runs := textRun.FindAllStringSubmatchIndex(paragraph, -1)
	if len(runs) < 2 {
		return paragraph
	}
	// Текст абзаца и то, в каком куске лежит каждый его байт
	var text strings.Builder
	var owner []int
	for i, m := range runs {
		piece := paragraph[m[4]:m[5]]
		text.WriteString(piece)
		for j := 0; j < len(piece); j++ {
			owner = append(owner, i)
		}
	}
	whole := text.String()

	// Переменная, которая начинается в одном куске и кончается в другом,
	// целиком уходит в тот, где начинается
	changed := false
	for _, m := range anyBracesRe.FindAllStringIndex(whole, -1) {
		first := owner[m[0]]
		if owner[m[1]-1] == first {
			continue
		}
		for i := m[0]; i < m[1]; i++ {
			owner[i] = first
		}
		changed = true
	}
	if !changed {
		return paragraph
	}

	pieces := make([]strings.Builder, len(runs))
	for i := 0; i < len(whole); i++ {
		_ = pieces[owner[i]].WriteByte(whole[i])
	}
	var b strings.Builder
	last := 0
	for i, m := range runs {
		b.WriteString(paragraph[last:m[0]])
		attrs := paragraph[m[2]:m[3]]
		if !strings.Contains(attrs, "xml:space") {
			// Без этого Word отбрасывает пробелы на концах куска
			attrs += ` xml:space="preserve"`
		}
		b.WriteString("<w:t" + attrs + ">" + pieces[i].String() + "</w:t>")
		last = m[1]
	}
	b.WriteString(paragraph[last:])
	return b.String()
}

// paragraphText возвращает текст абзаца без разметки.
func paragraphText(paragraph string) string {
	var b strings.Builder
	for _, m := range textRun.FindAllStringSubmatch(paragraph, -1) {
		b.WriteString(m[2])
	}
	return b.String()
}

// ─── Подстановка ───

// substitute заменяет каждую известную переменную её значением. Неизвестная
// остаётся как есть: её заметить проще, чем пустое место.
func substitute(content string, values map[string]string) string {
	return variableRe.ReplaceAllStringFunc(content, func(match string) string {
		name := strings.ToLower(variableRe.FindStringSubmatch(match)[1])
		value, ok := values[name]
		if !ok {
			return match
		}
		return escape(value)
	})
}

// escape делает значение безопасным для вставки в XML документа. Переносы
// строк становятся пробелами: значение — это одна строка текста.
func escape(value string) string {
	value = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(value)
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

// ─── Позиции счёта ───

func hasItemVariable(element string) bool {
	for _, m := range variableRe.FindAllStringSubmatch(element, -1) {
		if isItemVariable(strings.ToLower(m[1])) {
			return true
		}
	}
	return false
}

// repeatItemRows повторяет для каждой позиции счёта каждую строку таблицы,
// в которой стоят переменные позиции. Строка сохраняет оформление; строки
// вокруг неё — шапка, итоги — остаются на месте.
func repeatItemRows(content string, items []Item) string {
	var rows []span
	for _, row := range elementSpans(content, "w:tr") {
		if !hasItemVariable(content[row.start:row.end]) {
			continue
		}
		rows = append(rows, row)
	}
	// Во вложенных таблицах имеется в виду самая внутренняя строка с переменными
	var innermost []span
	for _, row := range rows {
		holdsAnother := false
		for _, other := range rows {
			if other != row && other.start >= row.start && other.end <= row.end {
				holdsAnother = true
				break
			}
		}
		if !holdsAnother {
			innermost = append(innermost, row)
		}
	}
	return rewrite(content, innermost, func(row string) string {
		var b strings.Builder
		for i, item := range items {
			b.WriteString(substitute(row, itemValues(i+1, item)))
		}
		return b.String()
	})
}

var runProperties = regexp.MustCompile(`(?s)<w:rPr>.*?</w:rPr>`)

// isItemsTablePlaceholder сообщает, состоит ли абзац из одной только
// {{items_table}}.
func isItemsTablePlaceholder(paragraph string) bool {
	m := variableRe.FindStringSubmatch(strings.TrimSpace(paragraphText(paragraph)))
	return m != nil && strings.ToLower(m[1]) == ItemsTable && m[0] == strings.TrimSpace(paragraphText(paragraph))
}

// insertItemsTable ставит таблицу счёта — позиции и итоги — на место
// абзаца с {{items_table}}. Текст таблицы берёт шрифт этого абзаца.
func insertItemsTable(content string, data Data) string {
	var placeholders []span
	for _, p := range leafSpans(elementSpans(content, "w:p")) {
		if isItemsTablePlaceholder(content[p.start:p.end]) {
			placeholders = append(placeholders, p)
		}
	}
	return rewrite(content, placeholders, func(paragraph string) string {
		// Таблица не может быть последней в ячейке или в документе: за
		// ней идёт пустой абзац
		return itemsTable(data, runProperties.FindString(paragraph)) + "<w:p/>"
	})
}

// Колонки построенной таблицы: заголовок, ширина в двадцатых долях пункта,
// выравнивание заголовка и значений и то, стоят ли значения посередине
// строки (длинное название переносится и остаётся вверху).
var tableColumns = []struct {
	title      string
	width      int
	titleAlign string
	align      string
	middle     bool
}{
	{"№", 454, "center", "center", true},
	{"Наименование услуги", 5530, "left", "left", false},
	{"Кол-во", 850, "center", "center", true},
	{"Цена, руб.", 1419, "center", "center", true},
	{"Сумма, руб.", 1416, "center", "center", true},
}

const (
	// tableBorderColor — цвет рамки построенной таблицы.
	tableBorderColor = "4A85B0"
	// tableIndent сдвигает таблицу, чтобы её рамка совпала с краем текста
	// страницы (отступ первой ячейки увёл бы её влево).
	tableIndent = 113
	// totalsLabelColumns: подписи итогов занимают столько колонок перед
	// последней; длинная подпись переносится внутри них.
	totalsLabelColumns = 2
)

// cellBorders описывает рамку ячейки: у каких её сторон есть линия.
type cellBorders struct{ top, left, bottom, right bool }

var framed = cellBorders{true, true, true, true}

func (c cellBorders) xml() string {
	side := func(name string, on bool) string {
		if !on {
			return `<w:` + name + ` w:val="nil"/>`
		}
		return `<w:` + name + ` w:val="single" w:sz="4" w:space="0" w:color="` + tableBorderColor + `"/>`
	}
	return `<w:tcBorders>` + side("top", c.top) + side("left", c.left) + side("bottom", c.bottom) + side("right", c.right) + `</w:tcBorders>`
}

// bold возвращает свойства куска текста с полужирным начертанием,
// добавленным там, где его требует схема документов Word: после стиля и
// шрифтов.
func bold(rPr string) string {
	if rPr == "" {
		return "<w:rPr><w:b/></w:rPr>"
	}
	if strings.Contains(rPr, "<w:b/>") || strings.Contains(rPr, "<w:b ") {
		return rPr
	}
	at := len("<w:rPr>")
	for _, leading := range []string{"<w:rStyle", "<w:rFonts"} {
		if i := strings.Index(rPr, leading); i >= 0 {
			if end := strings.Index(rPr[i:], ">"); end >= 0 && i+end+1 > at {
				at = i + end + 1
			}
		}
	}
	return rPr[:at] + "<w:b/>" + rPr[at:]
}

// itemsTable строит всю таблицу счёта: шапку, строку на каждую позицию и
// итоги под последней колонкой — сумму без НДС, НДС и сумму к оплате. Из
// итогов рамкой обведены только суммы.
//
// Рамка задаётся у каждой ячейки отдельно: у итогов её быть не должно, а
// рамка на всю таблицу провела бы линии и через них.
func itemsTable(data Data, rPr string) string {
	var b strings.Builder
	// Ширины колонок зафиксированы: какой бы длины ни было название,
	// таблица сохраняет форму, а название переносится внутри своей колонки.
	tableWidth := 0
	for _, col := range tableColumns {
		tableWidth += col.width
	}
	fmt.Fprintf(&b, `<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblInd w:w="%d" w:type="dxa"/><w:tblLayout w:type="fixed"/>`+
		`<w:tblCellMar><w:top w:w="11" w:type="dxa"/><w:bottom w:w="4" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid>`,
		tableWidth, tableIndent)
	for _, col := range tableColumns {
		fmt.Fprintf(&b, `<w:gridCol w:w="%d"/>`, col.width)
	}
	b.WriteString(`</w:tblGrid>`)

	// cell записывает ячейку шириной span колонок начиная с column.
	cell := func(column, span int, borders cellBorders, middle bool, text, align, properties string) {
		width := 0
		for _, col := range tableColumns[column : column+span] {
			width += col.width
		}
		fmt.Fprintf(&b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>`, width)
		if span > 1 {
			fmt.Fprintf(&b, `<w:gridSpan w:val="%d"/>`, span)
		}
		b.WriteString(borders.xml())
		if middle {
			b.WriteString(`<w:vAlign w:val="center"/>`)
		}
		fmt.Fprintf(&b, `</w:tcPr><w:p><w:pPr><w:spacing w:after="0"/><w:jc w:val="%s"/></w:pPr><w:r>%s<w:t xml:space="preserve">%s</w:t></w:r></w:p></w:tc>`,
			align, properties, escape(text))
	}

	// Шапка повторяется, когда таблица переходит на другую страницу
	b.WriteString(`<w:tr><w:trPr><w:tblHeader/></w:trPr>`)
	for i, col := range tableColumns {
		cell(i, 1, framed, true, col.title, col.titleAlign, rPr)
	}
	b.WriteString(`</w:tr>`)

	for i, item := range data.Items {
		values := itemValues(i+1, item)
		b.WriteString(`<w:tr>`)
		for column, name := range itemVariables {
			col := tableColumns[column]
			cell(column, 1, framed, col.middle, values[name], col.align, rPr)
		}
		b.WriteString(`</w:tr>`)
	}

	// Итоги: под первыми колонками пусто, перед последней — полужирная
	// подпись, в последней — сумма. У первой из этих строк остаётся линия,
	// закрывающая позиции счёта.
	last := len(tableColumns) - 1
	labelAt := last - totalsLabelColumns
	values := data.Values()
	rows := 0
	total := func(label, amount, properties string) {
		first := rows == 0
		rows++
		b.WriteString(`<w:tr>`)
		cell(0, labelAt, cellBorders{top: first}, false, "", "left", rPr)
		cell(labelAt, totalsLabelColumns, cellBorders{top: first, right: true}, true, label, "right", bold(rPr))
		cell(last, 1, framed, true, amount, tableColumns[last].align, properties)
		b.WriteString(`</w:tr>`)
	}
	total("Итого", values["subtotal"], rPr)
	if data.vatFraction() > 0 {
		total(values["vat_label"], values["vat_amount"], rPr)
		total("Всего (в том числе НДС):", values["total"], bold(rPr))
	} else {
		total(values["vat_label"], "—", rPr)
		total("Всего к оплате:", values["total"], bold(rPr))
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}
