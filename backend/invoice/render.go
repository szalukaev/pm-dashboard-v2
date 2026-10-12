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

// Limits of a template: a document of a page or two is tens of kilobytes;
// whatever is far above is not a template of an invoice.
const (
	MaxTemplateSize  = 5 << 20  // the .docx file
	maxUnpackedSize  = 60 << 20 // everything in it, unpacked
	maxDocumentParts = 2000     // files in the archive
)

// What can be wrong with a file offered as a template.
var (
	ErrNotDocx  = errors.New("the file is not a .docx document")
	ErrMacros   = errors.New("documents with macros are not accepted")
	ErrTooLarge = errors.New("the document is too large")
)

// Render fills the template with the data of an invoice and returns the
// resulting .docx.
func Render(template []byte, data Data) ([]byte, error) {
	archive, err := openTemplate(template)
	if err != nil {
		return nil, err
	}
	if len(data.Items) == 0 {
		// An invoice without lines is still an invoice for something
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

// openTemplate checks that the bytes are a Word document that may serve as
// a template.
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
	// The size in the header may lie: never read more than the limit
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

// textPart reports whether a file of the document holds text a variable may
// stand in: the body, the headers and footers, the notes.
func textPart(name string) bool {
	return textPartName.MatchString(name)
}

// fillPart substitutes the variables in one XML part of the document.
func fillPart(content string, data Data) string {
	content = joinSplitVariables(content)
	content = insertItemsTable(content, data)
	content = repeatItemRows(content, data.Items)
	return substitute(content, data.Values())
}

// ─── Finding elements ───

// span is an element of the XML text: [start, end) covers it with its tags.
type span struct {
	start, end int
	// nested: there is an element of the same kind inside it.
	nested bool
}

// elementSpans finds every <tag …>…</tag> in the text, however they are
// nested. Self-closing elements have no content and are skipped.
func elementSpans(content, tag string) []span {
	// \b keeps "w:p" from matching "w:pPr", and "w:tr" from "w:trPr"
	re := regexp.MustCompile(`<(/?)` + regexp.QuoteMeta(tag) + `\b[^>]*?(/?)>`)
	var spans []span
	var open []int // indexes into spans of the elements that are not closed yet
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
	// What was never closed is not an element
	closed := spans[:0]
	for _, s := range spans {
		if s.end > 0 {
			closed = append(closed, s)
		}
	}
	return closed
}

// rewrite replaces the given spans of the text with what replace returns
// for each. The spans must not overlap.
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

// leafSpans keeps the elements that hold no element of the same kind.
func leafSpans(spans []span) []span {
	var leaves []span
	for _, s := range spans {
		if !s.nested {
			leaves = append(leaves, s)
		}
	}
	return leaves
}

// ─── Variables split by Word ───

var (
	textRun     = regexp.MustCompile(`<w:t\b([^>]*)>([^<]*)</w:t>`)
	variableRe  = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)
	anyBracesRe = regexp.MustCompile(`\{\{([^{}]*)\}\}`)
)

// joinSplitVariables puts back together the variables Word has split.
//
// Word keeps a paragraph as pieces of text ("runs") and cuts them wherever
// it likes: after a spelling check or a change of font, "{{total}}" may be
// stored as "{{", "total" and "}}". Such a variable is moved whole into the
// piece it begins in; the formatting of that piece applies to it.
func joinSplitVariables(content string) string {
	return rewrite(content, leafSpans(elementSpans(content, "w:p")), joinInParagraph)
}

func joinInParagraph(paragraph string) string {
	runs := textRun.FindAllStringSubmatchIndex(paragraph, -1)
	if len(runs) < 2 {
		return paragraph
	}
	// The text of the paragraph, and which piece every byte of it is in
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

	// A variable that begins in one piece and ends in another goes whole to
	// the piece it begins in
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
		pieces[owner[i]].WriteByte(whole[i])
	}
	var b strings.Builder
	last := 0
	for i, m := range runs {
		b.WriteString(paragraph[last:m[0]])
		attrs := paragraph[m[2]:m[3]]
		if !strings.Contains(attrs, "xml:space") {
			// Without it Word drops the spaces at the ends of the piece
			attrs += ` xml:space="preserve"`
		}
		b.WriteString("<w:t" + attrs + ">" + pieces[i].String() + "</w:t>")
		last = m[1]
	}
	b.WriteString(paragraph[last:])
	return b.String()
}

// paragraphText returns the text of a paragraph without its markup.
func paragraphText(paragraph string) string {
	var b strings.Builder
	for _, m := range textRun.FindAllStringSubmatch(paragraph, -1) {
		b.WriteString(m[2])
	}
	return b.String()
}

// ─── Substitution ───

// substitute replaces every known variable by its value. A variable that is
// not known stays as it is: it is easier to notice than a blank.
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

// escape makes a value safe to put into the XML of the document. Line
// breaks become spaces: a value is one line of text.
func escape(value string) string {
	value = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(value)
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(value))
	return b.String()
}

// ─── Lines of the invoice ───

func hasItemVariable(element string) bool {
	for _, m := range variableRe.FindAllStringSubmatch(element, -1) {
		if isItemVariable(strings.ToLower(m[1])) {
			return true
		}
	}
	return false
}

// repeatItemRows repeats, for every line of the invoice, each table row
// that holds item variables. The row keeps its formatting; the rows around
// it — the header, the totals — stay where they are.
func repeatItemRows(content string, items []Item) string {
	var rows []span
	for _, row := range elementSpans(content, "w:tr") {
		if !hasItemVariable(content[row.start:row.end]) {
			continue
		}
		rows = append(rows, row)
	}
	// Of nested tables the innermost row with the variables is the one meant
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

// isItemsTablePlaceholder reports whether the paragraph is {{items_table}}
// and nothing else.
func isItemsTablePlaceholder(paragraph string) bool {
	m := variableRe.FindStringSubmatch(strings.TrimSpace(paragraphText(paragraph)))
	return m != nil && strings.ToLower(m[1]) == ItemsTable && m[0] == strings.TrimSpace(paragraphText(paragraph))
}

// insertItemsTable puts the table of the invoice — its lines and totals —
// in place of the paragraph that is {{items_table}}. The text of the table
// takes the font of that paragraph.
func insertItemsTable(content string, data Data) string {
	var placeholders []span
	for _, p := range leafSpans(elementSpans(content, "w:p")) {
		if isItemsTablePlaceholder(content[p.start:p.end]) {
			placeholders = append(placeholders, p)
		}
	}
	return rewrite(content, placeholders, func(paragraph string) string {
		// A table may not be the last thing in a cell or in the document:
		// an empty paragraph follows it
		return itemsTable(data, runProperties.FindString(paragraph)) + "<w:p/>"
	})
}

// Columns of the built table: title, width in twentieths of a point, how
// the title and the values are aligned, and whether the values stand in the
// middle of the row (a long name wraps and stays at the top).
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
	// tableBorderColor is the color of the frame of the built table.
	tableBorderColor = "4A85B0"
	// tableIndent moves the table so that its frame lines up with the text
	// of the page (the padding of the first cell would shift it left).
	tableIndent = 113
	// totalsLabelColumns: the labels of the totals take this many columns
	// before the last one; a long label wraps inside them.
	totalsLabelColumns = 2
)

// cellBorders describes the frame of a cell: which of its sides have a line.
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

// bold returns run properties with bold added in the place the schema of
// Word documents wants it: after the style and the fonts.
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

// itemsTable builds the whole table of an invoice: the header, a row for
// every line, and the totals under the last column — the subtotal, the VAT
// and the amount to pay. Of the totals only the amounts are framed.
//
// Every cell carries its own frame: the totals must have none, and a frame
// set for the whole table would draw lines through them.
func itemsTable(data Data, rPr string) string {
	var b strings.Builder
	// The widths of the columns are fixed: whatever the length of a name,
	// the table keeps its shape and the name wraps inside its column.
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

	// cell writes a cell over span columns starting at column.
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

	// The header is repeated when the table goes on to another page
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

	// The totals: nothing under the first columns, a bold label before the
	// last column and the amount in it. The first of these rows keeps the
	// line that closes the lines of the invoice.
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
