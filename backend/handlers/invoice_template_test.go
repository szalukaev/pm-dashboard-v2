package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testDocx builds a minimal Word document with the given text and files.
func testDocx(t *testing.T, text string, extra ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("word/document.xml")
	f.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` +
		text + `</w:t></w:r></w:p></w:body></w:document>`))
	for _, name := range extra {
		f, _ := w.Create(name)
		f.Write([]byte("x"))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func uploadRequest(t *testing.T, name string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	form.Close()
	r := httptest.NewRequest("POST", "/api/admin/invoice-template", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	return r
}

func newTemplateHandler(t *testing.T) *InvoiceTemplateHandler {
	t.Helper()
	var db *sql.DB // no database in tests: who uploaded is simply not kept
	templates := &InvoiceTemplates{DB: &db, Dir: t.TempDir(), BuiltIn: "../invoice/invoice_template.docx"}
	return &InvoiceTemplateHandler{DB: &db, Templates: templates}
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	return body.Error
}

func TestInvoiceTemplateUploadAndReset(t *testing.T) {
	h := newTemplateHandler(t)

	// At first the built-in template is in force
	builtIn, custom, err := h.Templates.Current()
	if err != nil || custom || len(builtIn) == 0 {
		t.Fatalf("built-in template: custom = %v, %d bytes, err = %v", custom, len(builtIn), err)
	}

	mine := testDocx(t, "Счёт {{invoice_num}} на {{total}}, {{compani}}, {{items_table}}")
	w := httptest.NewRecorder()
	h.Upload(w, uploadRequest(t, "Мой счёт.DOCX", mine))
	if w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	// A typo does not stop the upload; it is reported
	var answer struct {
		Report struct {
			Found   []string `json:"found"`
			Unknown []string `json:"unknown"`
		} `json:"report"`
	}
	json.Unmarshal(w.Body.Bytes(), &answer)
	if strings.Join(answer.Report.Unknown, ",") != "{{compani}}" || len(answer.Report.Found) != 3 {
		t.Errorf("report = %+v", answer.Report)
	}

	current, custom, _ := h.Templates.Current()
	if !custom || !bytes.Equal(current, mine) {
		t.Error("the uploaded template is not the one in force")
	}

	// The preview is made from the uploaded template
	w = httptest.NewRecorder()
	h.Preview(w, httptest.NewRequest("GET", "/api/admin/invoice-template/preview", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Disposition"), "filename*=UTF-8''") {
		t.Fatalf("preview: %d, headers %v", w.Code, w.Header())
	}
	archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil || len(archive.File) == 0 {
		t.Fatalf("the preview is not a document: %v", err)
	}

	// Back to the built-in one
	w = httptest.NewRecorder()
	h.Reset(w, httptest.NewRequest("DELETE", "/api/admin/invoice-template", nil))
	if current, custom, _ = h.Templates.Current(); custom || !bytes.Equal(current, builtIn) {
		t.Error("after a reset the built-in template must be in force")
	}
	// Resetting what is not there is not an error
	w = httptest.NewRecorder()
	h.Reset(w, httptest.NewRequest("DELETE", "/api/admin/invoice-template", nil))
	if w.Code != http.StatusOK {
		t.Errorf("a second reset: %d", w.Code)
	}
}

func TestInvoiceTemplateUploadRefused(t *testing.T) {
	good := testDocx(t, "{{total}} {{items_table}}")
	cases := []struct {
		name    string
		file    string
		content []byte
		want    string
	}{
		{"not a Word file by its name", "template.doc", good, "TEMPLATE_NOT_DOCX"},
		{"a text file named .docx", "template.docx", []byte("hello"), "TEMPLATE_NOT_DOCX"},
		{"a document with macros", "template.docx", testDocx(t, "{{total}}", "word/vbaProject.bin"), "TEMPLATE_HAS_MACROS"},
	}
	for _, c := range cases {
		h := newTemplateHandler(t)
		w := httptest.NewRecorder()
		h.Upload(w, uploadRequest(t, c.file, c.content))
		if w.Code != http.StatusBadRequest || errorCode(t, w) != c.want {
			t.Errorf("%s: %d %s, want 400 %s", c.name, w.Code, w.Body.String(), c.want)
		}
		// A refused file never replaces the template in force
		if _, custom, _ := h.Templates.Current(); custom {
			t.Errorf("%s: the refused file became the template", c.name)
		}
	}

	// No file at all
	h := newTemplateHandler(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/invoice-template", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	h.Upload(w, r)
	if w.Code != http.StatusBadRequest || errorCode(t, w) != "FILE_REQUIRED" {
		t.Errorf("no file: %d %s", w.Code, w.Body.String())
	}
}

func TestInvoiceTemplateInfo(t *testing.T) {
	h := newTemplateHandler(t)
	w := httptest.NewRecorder()
	h.Get(w, httptest.NewRequest("GET", "/api/admin/invoice-template", nil))
	var info struct {
		Custom    bool `json:"custom"`
		Variables []struct {
			Name    string `json:"name"`
			Group   string `json:"group"`
			Example string `json:"example"`
		} `json:"variables"`
		Report struct {
			Unknown  []string `json:"unknown"`
			Warnings []string `json:"warnings"`
		} `json:"report"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Custom || len(info.Variables) < 20 {
		t.Errorf("custom = %v, %d variables", info.Custom, len(info.Variables))
	}
	// The built-in template is a good one
	if len(info.Report.Unknown) != 0 || len(info.Report.Warnings) != 0 {
		t.Errorf("built-in template: %+v", info.Report)
	}
}
