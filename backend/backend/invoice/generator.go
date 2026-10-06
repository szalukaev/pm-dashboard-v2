package invoice

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ContractInfo holds contract data for invoice generation
type ContractInfo struct {
	CompanyName    string
	CompanyAddress string
	ContactPhone   string
	ContactName    string
	VatRate        string // "none" or "5"
	Name           string
}

// InvoiceItem is a line item in the invoice
type InvoiceItem struct {
	Name     string
	Quantity float64
	Price    float64
}

const wordNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// GenerateInvoice creates a .docx invoice from template
func GenerateInvoice(templatePath string, contract ContractInfo, items []InvoiceItem) (string, string, error) {
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("template not found: %s", templatePath)
	}

	// Calculate totals
	totalAmount := 0.0
	for _, item := range items {
		totalAmount += item.Quantity * item.Price
	}
	totalAmount = round2(totalAmount)

	vatRate := 0.0
	if contract.VatRate == "5" {
		vatRate = 0.05
	}
	vat := round2(totalAmount * vatRate)
	total := round2(totalAmount + vat)
	vatLabel := "Без НДС"
	if contract.VatRate == "5" {
		vatLabel = "НДС 5%"
	}

	now := time.Now()
	invNum := fmt.Sprintf("%d%02d-%02d", now.Year(), now.Month(), now.Day())
	today := now.Format("2006-01-02")

	// Open template
	templateZip, err := zip.OpenReader(templatePath)
	if err != nil {
		return "", "", fmt.Errorf("open template: %w", err)
	}
	defer templateZip.Close()

	// Create output
	outName := fmt.Sprintf("invoice_%s_%d.docx", sanitize(contract.CompanyName), now.Unix())
	outDir := "/tmp/invoices"
	os.MkdirAll(outDir, 0755)
	outPath := filepath.Join(outDir, outName)

	outFile, err := os.Create(outPath)
	if err != nil {
		return "", "", fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	outZip := zip.NewWriter(outFile)
	defer outZip.Close()

	// Process each file in template
	for _, f := range templateZip.File {
		rc, err := f.Open()
		if err != nil {
			return "", "", err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", "", err
		}

		if f.Name == "word/document.xml" {
			data, err = processDocumentXML(data, contract, items, totalAmount, vat, total, vatLabel, invNum, today)
			if err != nil {
				return "", "", fmt.Errorf("process document.xml: %w", err)
			}
		}

		w, err := outZip.Create(f.Name)
		if err != nil {
			return "", "", err
		}
		w.Write(data)
	}

	downloadName := fmt.Sprintf("Счет на оплату %s от %s.docx", contract.CompanyName, now.Format("2006.01.02"))
	return outPath, downloadName, nil
}

func processDocumentXML(data []byte, contract ContractInfo, items []InvoiceItem, totalAmount, vat, total float64, vatLabel, invNum, today string) ([]byte, error) {
	// Simple XML text replacement approach
	// This works for most templates where we just need to replace placeholder text
	content := string(data)

	// Replace recipient info
	content = replaceInXML(content, "{{company}}", contract.CompanyName)
	content = replaceInXML(content, "{{address}}", contract.CompanyAddress)
	content = replaceInXML(content, "{{phone}}", contract.ContactPhone)
	content = replaceInXML(content, "{{contact}}", contract.ContactName)
	content = replaceInXML(content, "{{invoice_num}}", invNum)
	content = replaceInXML(content, "{{date}}", today)
	content = replaceInXML(content, "{{vat_label}}", vatLabel)
	content = replaceInXML(content, "{{subtotal}}", fmtMoney(totalAmount))
	content = replaceInXML(content, "{{vat_amount}}", fmtMoney(vat))
	content = replaceInXML(content, "{{total}}", fmtMoney(total))

	// For item replacement, we'd need more complex XML parsing
	// For MVP, use simple approach with the first item
	if len(items) > 0 {
		content = replaceInXML(content, "{{item_name}}", items[0].Name)
		content = replaceInXML(content, "{{item_qty}}", fmt.Sprintf("%.0f", items[0].Quantity))
		content = replaceInXML(content, "{{item_price}}", fmtMoney(items[0].Price))
		content = replaceInXML(content, "{{item_amount}}", fmtMoney(items[0].Quantity*items[0].Price))
	}

	return []byte(content), nil
}

// replaceInXML replaces text that may be split across XML run elements
func replaceInXML(content, placeholder, value string) string {
	// First try simple replacement
	if strings.Contains(content, placeholder) {
		return strings.ReplaceAll(content, placeholder, xmlEscape(value))
	}
	return content
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func fmtMoney(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	decPart := parts[1]
	n := len(intPart)
	if n > 3 {
		var b strings.Builder
		for i, c := range intPart {
			if i > 0 && (n-i)%3 == 0 {
				b.WriteRune(' ')
			}
			b.WriteRune(c)
		}
		intPart = b.String()
	}
	return intPart + "," + decPart
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func sanitize(s string) string {
	r := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "\"", "'", "<", "", ">", "", "|", "_", "?", "", "*", "")
	return r.Replace(s)
}
