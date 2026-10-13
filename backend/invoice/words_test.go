package invoice

import "testing"

func TestAmountInWords(t *testing.T) {
	cases := map[float64]string{
		0:          "Ноль рублей 00 копеек",
		1:          "Один рубль 00 копеек",
		2:          "Два рубля 00 копеек",
		5:          "Пять рублей 00 копеек",
		11:         "Одиннадцать рублей 00 копеек",
		21:         "Двадцать один рубль 00 копеек",
		22.01:      "Двадцать два рубля 01 копейка",
		100.02:     "Сто рублей 02 копейки",
		112.14:     "Сто двенадцать рублей 14 копеек",
		1000:       "Одна тысяча рублей 00 копеек",
		2000:       "Две тысячи рублей 00 копеек",
		5000:       "Пять тысяч рублей 00 копеек",
		21000:      "Двадцать одна тысяча рублей 00 копеек",
		105000.5:   "Сто пять тысяч рублей 50 копеек",
		1000000:    "Один миллион рублей 00 копеек",
		2500001:    "Два миллиона пятьсот тысяч один рубль 00 копеек",
		1234567.89: "Один миллион двести тридцать четыре тысячи пятьсот шестьдесят семь рублей 89 копеек",
		3000000000: "Три миллиарда рублей 00 копеек",
		0.99:       "Ноль рублей 99 копеек",
		// Округление до копеек может перейти в рубли
		9.999: "Десять рублей 00 копеек",
	}
	for amount, want := range cases {
		if got := AmountInWords(amount); got != want {
			t.Errorf("%v: got %q, want %q", amount, got, want)
		}
	}
}

func TestFormatting(t *testing.T) {
	money := map[float64]string{0: "0,00", 150000: "150 000,00", 1234.56: "1 234,56", 999: "999,00", 1234567.5: "1 234 567,50"}
	for v, want := range money {
		if got := fmtMoney(v); got != want {
			t.Errorf("fmtMoney(%v) = %q, want %q", v, got, want)
		}
	}
	quantity := map[float64]string{1: "1", 2: "2", 12.5: "12,5", 0.25: "0,25", 3.10: "3,1"}
	for v, want := range quantity {
		if got := fmtQuantity(v); got != want {
			t.Errorf("fmtQuantity(%v) = %q, want %q", v, got, want)
		}
	}
	if got := dayRu("2026-12-31T00:00:00Z"); got != "31.12.2026" {
		t.Errorf("dayRu = %q", got)
	}
	if got := dayRu(""); got != "" {
		t.Errorf("dayRu of nothing = %q", got)
	}
}

func TestSampleValues(t *testing.T) {
	d := Sample()
	// 80 000 + 12,5 × 1 600 + 2 × 2 500,50 = 105 001; НДС 5 % = 5 250,05
	if d.Subtotal() != 105001 || d.VAT() != 5250.05 || d.Total() != 110251.05 {
		t.Fatalf("subtotal %v, vat %v, total %v", d.Subtotal(), d.VAT(), d.Total())
	}
	values := d.Values()
	want := map[string]string{
		"invoice_num":  "202610-09",
		"date":         "2026-10-09",
		"date_ru":      "09.10.2026",
		"date_text":    "9 октября 2026 г.",
		"subtotal":     "105 001,00",
		"vat_label":    "НДС 5%",
		"vat_amount":   "5 250,05",
		"total":        "110 251,05",
		"total_words":  "Сто десять тысяч двести пятьдесят один рубль 05 копеек",
		"items_count":  "3",
		"contract_end": "31.12.2026",
	}
	for name, value := range want {
		if values[name] != value {
			t.Errorf("{{%s}} = %q, want %q", name, values[name], value)
		}
	}
	// У каждой переменной из списка есть значение: ни одна не забыта
	for _, v := range Variables() {
		if v.Name != ItemsTable && v.Example == "" {
			t.Errorf("{{%s}} has no value for the sample invoice", v.Name)
		}
	}
	if d.FileName() != "Счет на оплату ООО «Ромашка» от 2026.10.09.docx" {
		t.Errorf("file name = %q", d.FileName())
	}
}
