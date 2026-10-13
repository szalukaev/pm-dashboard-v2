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

// docx собирает документ из частей; body — содержимое w:body.
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

// part читает один файл готового документа.
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
	// Разметка вокруг кусков сохранена: документ остаётся корректным XML
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

	// Шапка, три позиции, итоги
	if rows := strings.Count(doc, "<w:tr>") + strings.Count(doc, "<w:tr><w:trPr>"); rows != 5+3 {
		// каждая строка позиции подходит под оба шаблона, отсюда ещё 3
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
	// Позиции идут в своём порядке, шапка выше, итоги ниже
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
	// Шапка, три позиции по пять колонок и три строки итогов по три ячейки:
	// пустая, подпись, сумма
	if rows := strings.Count(doc, "</w:tr>"); rows != 7 {
		t.Errorf("the table has %d rows, want 7", rows)
	}
	if cells := strings.Count(doc, "</w:tc>"); cells != 29 {
		t.Errorf("the table has %d cells, want 29", cells)
	}
	for _, want := range []string{"Наименование услуги", "Кол-во", "Цена, руб.", "Сумма, руб.",
		"Доработка отчётов", "12,5", "20 000,00",
		"Итого", "105 001,00", "НДС 5%", "5 250,05", "Всего (в том числе НДС):", "После: 110 251,05"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the document lacks %q", want)
		}
	}
	// Сумма к оплате завершает таблицу
	if strings.LastIndex(doc[:strings.Index(doc, "</w:tbl>")], "110 251,05") < strings.Index(doc, "Всего (в том числе НДС):") {
		t.Error("the amount to pay must stand in the last row, after its label")
	}
	// Рамка задана у каждой ячейки отдельно. У ячейки позиции есть все
	// четыре стороны; в строке итогов обведена только сумма, у подписи
	// остаётся сторона, общая с суммой, а у первой из этих строк — линия,
	// закрывающая позиции счёта.
	line := func(side string) string {
		return `<w:` + side + ` w:val="single" w:sz="4" w:space="0" w:color="4A85B0"/>`
	}
	none := func(side string) string { return `<w:` + side + ` w:val="nil"/>` }
	borders := func(top, left, bottom, right string) string {
		return `<w:tcBorders>` + top + left + bottom + right + `</w:tcBorders>`
	}
	counts := map[string]struct {
		pattern string
		want    int
	}{
		"framed cells (5 header, 15 lines, 3 amounts)": {borders(line("top"), line("left"), line("bottom"), line("right")), 23},
		"the empty cell of the first row of totals":    {borders(line("top"), none("left"), none("bottom"), none("right")), 1},
		"the empty cells of the other rows":            {borders(none("top"), none("left"), none("bottom"), none("right")), 2},
		"the label of the first row of totals":         {borders(line("top"), none("left"), none("bottom"), line("right")), 1},
		"the labels of the other rows":                 {borders(none("top"), none("left"), none("bottom"), line("right")), 2},
	}
	for name, c := range counts {
		if got := strings.Count(doc, c.pattern); got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
	// Рамки на всю таблицу нет: она провела бы линии через итоги
	if strings.Contains(doc, "<w:tblBorders>") {
		t.Error("the frame must be set cell by cell, not for the table")
	}
	// Колонки имеют ширины счёта-образца и не растягиваются: длинное
	// название переносится внутри своей колонки
	if !strings.Contains(doc, `<w:tblW w:w="9669" w:type="dxa"/>`) || !strings.Contains(doc, `<w:tblLayout w:type="fixed"/>`) {
		t.Error("the table must have a fixed layout of the full width")
	}
	for _, width := range []string{"454", "5530", "850", "1419", "1416"} {
		if !strings.Contains(doc, `<w:gridCol w:w="`+width+`"/>`) {
			t.Errorf("a column of width %s is missing", width)
		}
	}
	// Подпись итога занимает две колонки перед суммой
	if got := strings.Count(doc, `<w:tcW w:w="2269" w:type="dxa"/><w:gridSpan w:val="2"/>`); got != 3 {
		t.Errorf("%d labels of totals have the width of two columns, want 3", got)
	}
	// Строки компактные, длинное название остаётся вверху своей строки
	if got := strings.Count(doc, `<w:spacing w:after="0"/>`); got != 29 {
		t.Errorf("%d cells have no space after the text, want all 29", got)
	}
	if got := strings.Count(doc, `<w:vAlign w:val="center"/>`); got != 29-3-3 {
		t.Errorf("%d cells are centered vertically, want all but the 3 names and the 3 empty cells", got)
	}
	// Текст таблицы берёт шрифт того места, где она стоит
	if got := strings.Count(doc, `w:ascii="Arial"`); got != 29 {
		t.Errorf("the font of the placeholder is used in %d cells, want all 29", got)
	}
	// Полужирное начертание — подписи итогов и сумма к оплате — идёт после
	// шрифтов, как требует схема Word
	if got := strings.Count(doc, `<w:rFonts w:ascii="Arial" w:hAnsi="Arial"/><w:b/><w:sz w:val="20"/>`); got != 4 {
		t.Errorf("bold is used %d times, want 4: three labels and the amount to pay", got)
	}
	// За таблицей идёт абзац
	if !strings.Contains(doc, "</w:tbl><w:p/>") {
		t.Error("a table must be followed by a paragraph")
	}
	if strings.Index(doc, "До") > strings.Index(doc, "<w:tbl>") || strings.Index(doc, "После") < strings.Index(doc, "</w:tbl>") {
		t.Error("the table is not where its variable stood")
	}
}

func TestRenderItemsTableWithoutVAT(t *testing.T) {
	data := Sample()
	data.VatRate = "none"
	out, err := Render(docx(t, para("{{items_table}}"), nil), data)
	if err != nil {
		t.Fatal(err)
	}
	doc := part(t, out, "word/document.xml")
	for _, want := range []string{"Итого", "Без НДС", "—", "Всего к оплате:", "105 001,00"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the document lacks %q", want)
		}
	}
	if strings.Contains(doc, "в том числе НДС") {
		t.Error("an invoice without VAT must not speak of VAT in the amount to pay")
	}
	wellFormed(t, "the table without VAT", doc)
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
	// Затрагиваются только части с текстом документа
	if got := part(t, out, "word/styles.xml"); got != `<w:styles>{{company}}</w:styles>` {
		t.Errorf("a part that is not text was changed: %s", got)
	}
	doc := part(t, out, "word/document.xml")
	if !strings.Contains(doc, `ООО &#34;Рога &amp; Копыта&#34; &lt;тест&gt;`) {
		t.Errorf("special characters are not escaped: %s", doc)
	}
	// То, что переменной не является, остаётся, чтобы ошибка была видна
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
	// Таблица позиций стоит в ячейке таблицы-макета: повторяется только её собственная строка
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

	// Опечатки, нет места для позиций, нет суммы к оплате
	report, _ = Inspect(docx(t, para("{{compani}} {{сумма}} {{company}}"), nil))
	if strings.Join(report.Unknown, ",") != "{{compani}},{{сумма}}" {
		t.Errorf("unknown = %v", report.Unknown)
	}
	if strings.Join(report.Warnings, ",") != WarnNoItems+","+WarnNoTotal {
		t.Errorf("warnings = %v", report.Warnings)
	}

	// Переменная таблицы стоит в одной строке с другим текстом
	report, _ = Inspect(docx(t, para("Позиции: {{items_table}}")+para("{{total}}"), nil))
	if strings.Join(report.Warnings, ",") != WarnNoItems+","+WarnItemsTableNotAlone {
		t.Errorf("warnings = %v", report.Warnings)
	}

	// Переменные позиции вне таблицы: показана была бы только одна позиция
	report, _ = Inspect(docx(t, para("{{item_name}} {{total}}"), nil))
	if strings.Join(report.Warnings, ",") != WarnNoItems+","+WarnItemsOutsideTable {
		t.Errorf("warnings = %v", report.Warnings)
	}

	// Переменной таблицы, стоящей в строке одной, позициям достаточно
	report, _ = Inspect(docx(t, para("{{items_table}}"), nil))
	if len(report.Warnings) != 0 {
		t.Errorf("warnings = %v", report.Warnings)
	}
}

func TestRefusedFiles(t *testing.T) {
	if _, err := Inspect([]byte("just some text, not an archive")); err != ErrNotDocx {
		t.Errorf("plain text: err = %v, want ErrNotDocx", err)
	}

	// Архив, который не является документом Word
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
	// В счёте есть все позиции, а не только первая
	for _, want := range []string{"Сопровождение системы, октябрь 2026", "Доработка отчётов", "Выезд специалиста", "110 251,05"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the invoice lacks %q", want)
		}
	}
}

// wellFormed проваливает тест, если XML не читается до конца.
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
