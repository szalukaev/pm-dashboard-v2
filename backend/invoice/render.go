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
	content = insertItemsTable(content, data.Items)
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

// insertItemsTable puts a table of the lines in place of the paragraph that
// is {{items_table}}. The text of the table takes the font of that paragraph.
func insertItemsTable(content string, items []Item) string {
	var placeholders []span
	for _, p := range leafSpans(elementSpans(content, "w:p")) {
		if isItemsTablePlaceholder(content[p.start:p.end]) {
			placeholders = append(placeholders, p)
		}
	}
	return rewrite(content, placeholders, func(paragraph string) string {
		// A table may not be the last thing in a cell or in the document:
		// an empty paragraph follows it
		return itemsTable(items, runProperties.FindString(paragraph)) + "<w:p/>"
	})
}

// Columns of the built table: title, width in twentieths of a point (the
// whole is the text width of an A4 page with usual margins), alignment.
var tableColumns = []struct {
	title string
	width int
	align string
}{
	{"№", 600, "center"},
	{"Наименование", 4800, "left"},
	{"Кол-во", 1100, "right"},
	{"Цена", 1500, "right"},
	{"Сумма", 1638, "right"},
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

func itemsTable(items []Item, rPr string) string {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="5000" w:type="pct"/><w:tblBorders>`)
	for _, side := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		fmt.Fprintf(&b, `<w:%s w:val="single" w:sz="4" w:space="0" w:color="auto"/>`, side)
	}
	b.WriteString(`</w:tblBorders></w:tblPr><w:tblGrid>`)
	for _, col := range tableColumns {
		fmt.Fprintf(&b, `<w:gridCol w:w="%d"/>`, col.width)
	}
	b.WriteString(`</w:tblGrid>`)

	cell := func(column int, text, properties, shading string) {
		col := tableColumns[column]
		fmt.Fprintf(&b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>%s</w:tcPr><w:p><w:pPr><w:jc w:val="%s"/></w:pPr><w:r>%s<w:t xml:space="preserve">%s</w:t></w:r></w:p></w:tc>`,
			col.width, shading, col.align, properties, escape(text))
	}

	// The header is repeated when the table goes on to another page
	b.WriteString(`<w:tr><w:trPr><w:tblHeader/></w:trPr>`)
	for i, col := range tableColumns {
		cell(i, col.title, bold(rPr), `<w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/>`)
	}
	b.WriteString(`</w:tr>`)

	for i, item := range items {
		values := itemValues(i+1, item)
		b.WriteString(`<w:tr>`)
		for column, name := range itemVariables {
			cell(column, values[name], rPr, "")
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>`)
	return b.String()
}
