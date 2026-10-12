package invoice

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"
)

const docHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`
const docTail = `</w:body></w:document>`

// docx builds a document from its parts; body is the content of w:body.
func docx(t *testing.T, body string, extra map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"word/document.xml":   docHead + body + docTail,
	}
	for name, content := range extra {
		parts[name] = content
	}
	for name, content := range parts {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(content))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// part reads one file of a rendered document.
func part(t *testing.T, document []byte, name string) string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(document), int64(len(document)))
	if err != nil {
		t.Fatalf("the result is not a zip archive: %v", err)
	}
	for _, f := range archive.File {
		if f.Name == name {
			rc, _ := f.Open()
			defer rc.Close()
			content, _ := io.ReadAll(rc)
			return string(content)
		}
	}
	t.Fatalf("%s is missing in the result", name)
	return ""
}

func para(text string) string {
	return `<w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>`
}

func cell(text string) string {
	return `<w:tc><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:tc>`
}

func TestRenderSimpleVariables(t *testing.T) {
	template := docx(t, para("Счёт № {{invoice_num}} от {{date_ru}}")+para("{{company}}, ИНН {{ inn }}")+para("Итого: {{total}} ({{total_words}})"), nil)
	out, err := Render(template, Sample())
	if err != nil {
		t.Fatal(err)
	}
	doc := part(t, out, "word/document.xml")
	for _, want := range []string{
		"Счёт № 202610-09 от 09.10.2026",
		"ООО «Ромашка», ИНН 6671234567",
		"Итого: 110 251,05 (Сто десять тысяч двести пятьдесят один рубль 05 копеек)",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("the document lacks %q", want)
		}
	}
	if strings.Contains(doc, "{{") {
		t.Errorf("a variable was left unfilled: %s", doc)
	}
}

func TestRenderVariableSplitByWord(t *testing.T) {
	// What Word makes of "Итого: {{total}} руб." after a spelling check
	split := `<w:p><w:r><w:t xml:space="preserve">Итого: {</w:t></w:r><w:proofErr w:type="spellStart"/>` +
		`<w:r><w:rPr><w:b/></w:rPr><w:t>{to</w:t></w:r><w:r><w:t>tal</w:t></w:r><w:proofErr w:type="spellEnd"/>` +
		`<w:r><w:t xml:space="preserve">}} руб.</w:t></w:r></w:p>`
	out, err := Render(docx(t, split, nil), Sample())
	if err != nil {
		t.Fatal(err)
	}
	doc := part(t, out, "word/document.xml")
	if text := paragraphText(doc); text != "Итого: 110 251,05 руб." {
		t.Errorf("text = %q, want the variable filled", text)
	}
	// The markup around the pieces is kept: the document stays well formed
	if strings.Count(doc, "<w:r>") != 4 || !strings.Contains(doc, `<w:proofErr w:type="spellEnd"/>`) {
		t.Errorf("the markup of the paragraph was damaged: %s", doc)
	}
}

func TestRenderRepeatsItemRow(t *testing.T) {
	table := `<w:tbl>` +
		`<w:tr>` + cell("№") + cell("Наименование") + cell("Сумма") + `</w:tr>` +
		`<w:tr><w:trPr><w:cantSplit/></w:trPr>` + cell("{{item_no}}") + cell("{{item_name}}") + cell("{{item_qty}} × {{item_price}} = {{item_amount}}") + `</w:tr>` +
		`<w:tr>` + cell("") + cell("Итого") + cell("{{subtotal}}") + `</w:tr>` +
		`</w:tbl>`
	out, err := Render(docx(t, para("Позиций: {{items_count}}")+table, nil), Sample())
	if err != nil {
		t.Fatal(err)
	}
	doc := part(t, out, "word/document.xml")

	// Header, three lines, totals
	if rows := strings.Count(doc, "<w:tr>") + strings.Count(doc, "<w:tr><w:trPr>"); rows != 5+3 {
		// each item row matches both patterns, hence 3 more
		t.Errorf("rows counted %d", rows)
	}
	if got := strings.Count(doc, "<w:cantSplit/>"); got != 3 {
		t.Errorf("the item row was repeated %d times, want 3 with its formatting", got)
	}
	for _, want := range []string{
		"Сопровождение системы, октябрь 2026", "1 × 80 000,00 = 80 000,00",
		"Доработка отчётов", "12,5 × 1 600,00 = 20 000,00",
		"Выезд специалиста", "2 × 2 500,50 = 5 001,00",
		"105 001,00", "Позиций: 3",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("the document lacks %q", want)
		}
	}
	// The lines keep their order, the header is above and the totals below
	order := []string{"Наименование", "Сопровождение системы", "Доработка отчётов", "Выезд специалиста", "Итого"}
	last := -1
	for _, text := range order {
		at := strings.Index(doc, text)
		if at <= last {
			t.Fatalf("%q is out of order", text)
		}
		last = at
	}
}

func TestRenderItemsTable(t *testing.T) {
	placeholder := `<w:p><w:pPr><w:jc w:val="left"/></w:pPr><w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial"/><w:sz w:val="20"/></w:rPr><w:t>{{items_table}}</w:t></w:r></w:p>`
	out, err := Render(docx(t, para("До")+placeholder+para("После: {{total}}"), nil), Sample())
	if err != nil {
		t.Fatal(err)
	}
	doc := part(t, out, "word/document.xml")
	if strings.Contains(doc, "items_table") {
		t.Fatal("the variable of the table was left in the document")
	}
	if strings.Count(doc, "<w:tbl>") != 1 || strings.Count(doc, "</w:tbl>") != 1 {
		t.Fatalf("expected one table: %s", doc)
	}
	// A header and three lines, five columns each
	if rows := strings.Count(doc, "</w:tr>"); rows != 4 {
		t.Errorf("the table has %d rows, want 4", rows)
	}
	if cells := strings.Count(doc, "</w:tc>"); cells != 20 {
		t.Errorf("the table has %d cells, want 20", cells)
	}
	for _, want := range []string{"Наименование", "Кол-во", "Доработка отчётов", "12,5", "20 000,00", "После: 110 251,05"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the document lacks %q", want)
		}
	}
	// The text of the table takes the font of the place it stands in
	if got := strings.Count(doc, `w:ascii="Arial"`); got != 20 {
		t.Errorf("the font of the placeholder is used in %d cells, want all 20", got)
	}
	// Bold in the header goes after the fonts, as the schema of Word wants
	if !strings.Contains(doc, `<w:rFonts w:ascii="Arial" w:hAnsi="Arial"/><w:b/><w:sz w:val="20"/>`) {
		t.Error("the header is not bold, or bold is in the wrong place")
	}
	// A paragraph follows the table
	if !strings.Contains(doc, "</w:tbl><w:p/>") {
		t.Error("a table must be followed by a paragraph")
	}
	if strings.Index(doc, "До") > strings.Index(doc, "<w:tbl>") || strings.Index(doc, "После") < strings.Index(doc, "</w:tbl>") {
		t.Error("the table is not where its variable stood")
	}
}

func TestRenderHeadersFootersAndEscaping(t *testing.T) {
	header := `<?xml version="1.0"?><w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` + para("Счёт {{invoice_num}}") + `</w:hdr>`
	template := docx(t, para("{{company}} / {{mistake}}"), map[string]string{
		"word/header1.xml": header,
		"word/styles.xml":  `<w:styles>{{company}}</w:styles>`,
	})
	data := Sample()
	data.Company = `ООО "Рога & Копыта" <тест>`
	data.Address = "строка один\nстрока два"
	out, err := Render(template, data)
	if err != nil {
		t.Fatal(err)
	}
	if got := part(t, out, "word/header1.xml"); !strings.Contains(got, "Счёт 202610-09") {
		t.Errorf("the header was not filled: %s", got)
	}
	// Only the parts that hold text of the document are touched
	if got := part(t, out, "word/styles.xml"); got != `<w:styles>{{company}}</w:styles>` {
		t.Errorf("a part that is not text was changed: %s", got)
	}
	doc := part(t, out, "word/document.xml")
	if !strings.Contains(doc, `ООО &#34;Рога &amp; Копыта&#34; &lt;тест&gt;`) {
		t.Errorf("special characters are not escaped: %s", doc)
	}
	// What is not a variable stays, so that the mistake is seen
	if !strings.Contains(doc, "{{mistake}}") {
		t.Error("an unknown variable must stay as it is")
	}
}

func TestRenderWithoutItems(t *testing.T) {
	data := Sample()
	data.Items = nil
	table := `<w:tbl><w:tr>` + cell("{{item_no}}") + cell("{{item_name}}") + `</w:tr></w:tbl>`
	out, err := Render(docx(t, table, nil), data)
	if err != nil {
		t.Fatal(err)
	}
	doc := part(t, out, "word/document.xml")
	if strings.Count(doc, "</w:tr>") != 1 || !strings.Contains(doc, "Договор сопровождения № 12") {
		t.Errorf("an invoice without lines must show one line of the contract: %s", doc)
	}
}

func TestRenderNestedTable(t *testing.T) {
	// The items table sits in a cell of a layout table: only its own row is repeated
	inner := `<w:tbl><w:tr>` + cell("{{item_name}}") + `</w:tr></w:tbl>`
	outer := `<w:tbl><w:tr><w:tc>` + para("Шапка") + inner + `<w:p/></w:tc></w:tr></w:tbl>`
	out, err := Render(docx(t, outer, nil), Sample())
	if err != nil {
		t.Fatal(err)
	}
	doc := part(t, out, "word/document.xml")
	if got := strings.Count(doc, "Шапка"); got != 1 {
		t.Errorf("the outer row was repeated: the header appears %d times", got)
	}
	if got := strings.Count(doc, "</w:tr>"); got != 4 {
		t.Errorf("rows = %d, want the outer one and three lines", got)
	}
}

func TestInspect(t *testing.T) {
	row := `<w:tbl><w:tr>` + cell("{{item_name}}") + cell("{{item_amount}}") + `</w:tr></w:tbl>`

	report, err := Inspect(docx(t, para("{{company}} {{Total}} {{ date }}")+row, nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Found, ",") != "date,total,company,item_name,item_amount" {
		t.Errorf("found = %v", report.Found)
	}
	if len(report.Unknown) != 0 || len(report.Warnings) != 0 {
		t.Errorf("a good template gives unknown %v and warnings %v", report.Unknown, report.Warnings)
	}

	// Typos, no place for the lines, no amount to pay
	report, _ = Inspect(docx(t, para("{{compani}} {{сумма}} {{company}}"), nil))
	if strings.Join(report.Unknown, ",") != "{{compani}},{{сумма}}" {
		t.Errorf("unknown = %v", report.Unknown)
	}
	if strings.Join(report.Warnings, ",") != WarnNoItems+","+WarnNoTotal {
		t.Errorf("warnings = %v", report.Warnings)
	}

	// The table variable shares its line with other text
	report, _ = Inspect(docx(t, para("Позиции: {{items_table}}")+para("{{total}}"), nil))
	if strings.Join(report.Warnings, ",") != WarnNoItems+","+WarnItemsTableNotAlone {
		t.Errorf("warnings = %v", report.Warnings)
	}

	// Item variables outside a table: only one line would be shown
	report, _ = Inspect(docx(t, para("{{item_name}} {{total}}"), nil))
	if strings.Join(report.Warnings, ",") != WarnNoItems+","+WarnItemsOutsideTable {
		t.Errorf("warnings = %v", report.Warnings)
	}

	// The table variable alone on its line is all the lines need
	report, _ = Inspect(docx(t, para("{{items_table}}")+para("{{total_words}}"), nil))
	if len(report.Warnings) != 0 {
		t.Errorf("warnings = %v", report.Warnings)
	}
}

func TestRefusedFiles(t *testing.T) {
	if _, err := Inspect([]byte("just some text, not an archive")); err != ErrNotDocx {
		t.Errorf("plain text: err = %v, want ErrNotDocx", err)
	}

	// An archive that is not a Word document
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("readme.txt")
	f.Write([]byte("hello"))
	w.Close()
	if _, err := Inspect(buf.Bytes()); err != ErrNotDocx {
		t.Errorf("a zip without a document: err = %v, want ErrNotDocx", err)
	}

	withMacros := docx(t, para("{{total}}"), map[string]string{"word/vbaProject.bin": "macros"})
	if _, err := Inspect(withMacros); err != ErrMacros {
		t.Errorf("a document with macros: err = %v, want ErrMacros", err)
	}
	if _, err := Render(withMacros, Sample()); err != ErrMacros {
		t.Errorf("rendering a document with macros: err = %v, want ErrMacros", err)
	}

	if _, err := Inspect(make([]byte, MaxTemplateSize+1)); err != ErrTooLarge {
		t.Errorf("a file above the limit: err = %v, want ErrTooLarge", err)
	}
}

func TestBuiltInTemplate(t *testing.T) {
	template, err := os.ReadFile("invoice_template.docx")
	if err != nil {
		t.Skip("the built-in template is not next to the tests")
	}
	report, err := Inspect(template)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unknown) != 0 || len(report.Warnings) != 0 {
		t.Errorf("the built-in template: unknown %v, warnings %v", report.Unknown, report.Warnings)
	}

	out, err := Render(template, Sample())
	if err != nil {
		t.Fatal(err)
	}
	doc := part(t, out, "word/document.xml")
	if strings.Contains(doc, "{{") {
		t.Error("the built-in template has a variable that was not filled")
	}
	// Every line of the invoice is there, not only the first
	for _, want := range []string{"Сопровождение системы, октябрь 2026", "Доработка отчётов", "Выезд специалиста", "110 251,05"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the invoice lacks %q", want)
		}
	}
}

// wellFormed fails the test when the XML cannot be read to its end.
func wellFormed(t *testing.T, name, content string) {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		if _, err := decoder.Token(); err == io.EOF {
			return
		} else if err != nil {
			t.Fatalf("%s is not well-formed XML after rendering: %v", name, err)
		}
	}
}

func TestRenderedXMLIsWellFormed(t *testing.T) {
	split := `<w:p><w:r><w:t>{{com</w:t></w:r><w:r><w:t>pany}} &amp; {{</w:t></w:r><w:r><w:t>total}}</w:t></w:r></w:p>`
	row := `<w:tbl><w:tr>` + cell("{{item_name}}") + cell("{{item_amount}}") + `</w:tr></w:tbl>`
	templates := map[string][]byte{
		"split variables and a row": docx(t, split+row, nil),
		"the built table":           docx(t, para("{{items_table}}")+para("{{total}}"), nil),
	}
	if builtIn, err := os.ReadFile("invoice_template.docx"); err == nil {
		templates["the built-in template"] = builtIn
	}
	data := Sample()
	data.Company = `ООО "A&B" <x>`
	data.Items[0].Name = `Работы по договору "<срочно>" & прочее`
	for name, template := range templates {
		out, err := Render(template, data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		wellFormed(t, name, part(t, out, "word/document.xml"))
	}
}
