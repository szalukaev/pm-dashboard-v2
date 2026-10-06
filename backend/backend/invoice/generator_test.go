package invoice

import (
	"os"
	"testing"
)

func TestFmtMoney(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0,00"},
		{150000, "150 000,00"},
		{1234.56, "1 234,56"},
		{999, "999,00"},
	}
	for _, tt := range tests {
		result := fmtMoney(tt.input)
		if result != tt.expected {
			t.Errorf("fmtMoney(%f) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestRound2(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
	}
	for _, tt := range tests {
		result := round2(tt.input)
		if result != tt.expected {
			t.Errorf("round2(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ООО Рога и Копыта", "ООО_Рога_и_Копыта"},
		{"Test/Company", "Test_Company"},
		{"Normal", "Normal"},
	}
	for _, tt := range tests {
		result := sanitize(tt.input)
		if result != tt.expected {
			t.Errorf("sanitize(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateInvoice_TemplateNotFound(t *testing.T) {
	_, _, err := GenerateInvoice("/nonexistent/template.docx", ContractInfo{}, []InvoiceItem{})
	if err == nil {
		t.Error("Expected error for missing template, got nil")
	}
}

func TestGenerateInvoice_WithTemplate(t *testing.T) {
	templatePath := "invoice_template.docx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("Template file not found, skipping")
	}

	contract := ContractInfo{
		CompanyName:    "Test Company",
		CompanyAddress: "Test Address",
		ContactPhone:   "+7 999 123 45 67",
		ContactName:    "Иванов И.И.",
		VatRate:        "5",
		Name:           "Test Contract",
	}
	items := []InvoiceItem{
		{Name: "Услуга 1", Quantity: 2, Price: 50000},
		{Name: "Услуга 2", Quantity: 1, Price: 30000},
	}

	outPath, downloadName, err := GenerateInvoice(templatePath, contract, items)
	if err != nil {
		t.Fatalf("GenerateInvoice failed: %v", err)
	}
	if outPath == "" {
		t.Error("Expected non-empty output path")
	}
	if downloadName == "" {
		t.Error("Expected non-empty download name")
	}

	// Cleanup
	os.Remove(outPath)
}
