package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pm-dashboard/invoice"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"
)

// docxContentType — тип содержимого документа Word.
const docxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// InvoiceTemplates хранит шаблон, по которому формируются счета:
// загруженный администратором или встроенный. Шаблон один на всю установку.
type InvoiceTemplates struct {
	DB **sql.DB
	// Dir — где лежит загруженный шаблон (каталог данных: он переживает
	// пересборку контейнера).
	Dir string
	// BuiltIn — путь к шаблону из поставки.
	BuiltIn string
}

const (
	customTemplateFile = "invoice_template.docx"
	// templateSettingsKey: что известно о загруженном шаблоне.
	templateSettingsKey = "invoice_template"
)

// templateInfo хранится в настройках рядом с загруженным файлом.
type templateInfo struct {
	FileName   string    `json:"file_name"`
	UploadedAt time.Time `json:"uploaded_at"`
	UploadedBy string    `json:"uploaded_by"`
	Size       int       `json:"size"`
}

func (t *InvoiceTemplates) customPath() string {
	return filepath.Join(t.Dir, customTemplateFile)
}

// Current возвращает действующий шаблон и то, загруженный ли он. Если
// загруженный шаблон не читается, берётся встроенный: счета из-за этого
// останавливаться не должны.
func (t *InvoiceTemplates) Current() (template []byte, custom bool, err error) {
	if content, readErr := os.ReadFile(t.customPath()); readErr == nil {
		return content, true, nil
	} else if !errors.Is(readErr, os.ErrNotExist) {
		slog.Warn("The uploaded invoice template cannot be read, the built-in one is used", "error", readErr)
	}
	content, err := os.ReadFile(t.BuiltIn)
	return content, false, err
}

func (t *InvoiceTemplates) info() *templateInfo {
	if *t.DB == nil {
		return nil
	}
	var raw []byte
	if (*t.DB).QueryRow(`SELECT value FROM settings WHERE key = $1`, templateSettingsKey).Scan(&raw) != nil {
		return nil
	}
	var info templateInfo
	if json.Unmarshal(raw, &info) != nil {
		return nil
	}
	return &info
}

// Save заменяет загруженный шаблон. Файл пишется рядом со своим местом и
// затем переносится на него: при сбое посередине остаётся прежний.
func (t *InvoiceTemplates) Save(content []byte, info templateInfo) error {
	if err := os.MkdirAll(t.Dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(t.Dir, "invoice_template_*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), t.customPath()); err != nil {
		return err
	}
	if *t.DB != nil {
		data, _ := json.Marshal(info)
		if _, err := (*t.DB).Exec(`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, templateSettingsKey, data); err != nil {
			slog.Warn("Could not save who uploaded the invoice template", "error", err)
		}
	}
	return nil
}

// Reset удаляет загруженный шаблон: снова действует встроенный.
func (t *InvoiceTemplates) Reset() error {
	if err := os.Remove(t.customPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if *t.DB != nil {
		if _, err := (*t.DB).Exec(`DELETE FROM settings WHERE key = $1`, templateSettingsKey); err != nil {
			slog.Warn("Database write failed", "at", "Reset", "error", err)
		}
	}
	return nil
}

// InvoiceTemplateHandler обслуживает экран, на котором администратор
// управляет шаблоном счёта.
type InvoiceTemplateHandler struct {
	DB        **sql.DB
	Templates *InvoiceTemplates
}

// Get возвращает, какой шаблон действует, что нашла его проверка и все
// переменные, которые может использовать шаблон.
func (h *InvoiceTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	template, custom, err := h.Templates.Current()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "TEMPLATE_NOT_FOUND")
		return
	}
	result := map[string]interface{}{
		"custom":    custom,
		"size":      len(template),
		"max_size":  invoice.MaxTemplateSize,
		"variables": invoice.Variables(),
	}
	if custom {
		if info := h.Templates.info(); info != nil {
			result["file_name"], result["uploaded_at"], result["uploaded_by"] = info.FileName, info.UploadedAt, info.UploadedBy
		}
	}
	if report, err := invoice.Inspect(template); err == nil {
		result["report"] = report
	}
	utils.JSON(w, http.StatusOK, result)
}

// templateError отвечает причиной, по которой файл не может быть шаблоном.
func templateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, invoice.ErrMacros):
		utils.Error(w, http.StatusBadRequest, "TEMPLATE_HAS_MACROS")
	case errors.Is(err, invoice.ErrTooLarge):
		utils.Error(w, http.StatusBadRequest, "TEMPLATE_TOO_LARGE")
	default:
		utils.Error(w, http.StatusBadRequest, "TEMPLATE_NOT_DOCX")
	}
}

// Upload заменяет шаблон загруженным файлом. Файл, который не может быть
// шаблоном, не принимается, и действующий шаблон остаётся.
func (h *InvoiceTemplateHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// Чуть больше предела самого файла: форма добавляет свою обвязку
	r.Body = http.MaxBytesReader(w, r.Body, invoice.MaxTemplateSize+(1<<20))
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			utils.Error(w, http.StatusBadRequest, "TEMPLATE_TOO_LARGE")
			return
		}
		utils.Error(w, http.StatusBadRequest, "FILE_REQUIRED")
		return
	}
	defer file.Close()

	if !strings.EqualFold(filepath.Ext(header.Filename), ".docx") {
		utils.Error(w, http.StatusBadRequest, "TEMPLATE_NOT_DOCX")
		return
	}
	content, err := io.ReadAll(io.LimitReader(file, invoice.MaxTemplateSize+1))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "FILE_REQUIRED")
		return
	}
	report, err := invoice.Inspect(content)
	if err != nil {
		templateError(w, err)
		return
	}
	// Шаблон должен выдержать и заполнение, а не только чтение
	if _, err := invoice.Render(content, invoice.Sample()); err != nil {
		templateError(w, err)
		return
	}

	info := templateInfo{FileName: filepath.Base(header.Filename), UploadedAt: time.Now(), Size: len(content)}
	if *h.DB != nil {
		if err := (*h.DB).QueryRow(`SELECT username FROM users WHERE id = $1`, middleware.GetUserID(r)).Scan(&info.UploadedBy); err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("Database query failed", "at", "Upload", "error", err)
		}
	}
	if err := h.Templates.Save(content, info); err != nil {
		slog.Error("Could not save the invoice template", "error", err)
		utils.Error(w, http.StatusInternalServerError, "TEMPLATE_SAVE_FAILED")
		return
	}
	slog.Info("Invoice template replaced", "by", info.UploadedBy, "file", info.FileName, "size", info.Size)
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "report": report})
}

// Reset возвращает встроенный шаблон.
func (h *InvoiceTemplateHandler) Reset(w http.ResponseWriter, r *http.Request) {
	if err := h.Templates.Reset(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "TEMPLATE_SAVE_FAILED")
		return
	}
	utils.Success(w)
}

// File отдаёт действующий шаблон: чтобы посмотреть его или взять встроенный
// за основу своего.
func (h *InvoiceTemplateHandler) File(w http.ResponseWriter, r *http.Request) {
	template, custom, err := h.Templates.Current()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "TEMPLATE_NOT_FOUND")
		return
	}
	name := "Шаблон счёта (встроенный).docx"
	if custom {
		name = "Шаблон счёта.docx"
		if info := h.Templates.info(); info != nil && info.FileName != "" {
			name = info.FileName
		}
	}
	serveDocx(w, name, template)
}

// Preview отдаёт счёт, собранный по действующему шаблону на вымышленных
// данных: так будет выглядеть настоящий счёт.
func (h *InvoiceTemplateHandler) Preview(w http.ResponseWriter, r *http.Request) {
	template, _, err := h.Templates.Current()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "TEMPLATE_NOT_FOUND")
		return
	}
	document, err := invoice.Render(template, invoice.Sample())
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DOCX_GENERATION_FAILED")
		return
	}
	serveDocx(w, "Пробный счёт.docx", document)
}

// serveDocx отдаёт документ Word на скачивание. Имя может быть русским:
// filename* несёт его в закодированном виде, filename — запасной вариант
// для тех, кто filename* не понимает.
func serveDocx(w http.ResponseWriter, name string, content []byte) {
	w.Header().Set("Content-Type", docxContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"document.docx\"; filename*=UTF-8''%s", url.PathEscape(name)))
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(content)
}
