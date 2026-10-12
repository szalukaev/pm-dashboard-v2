// Package invoice fills a .docx template with the data of an invoice.
//
// The template is an ordinary Word document. What changes from invoice to
// invoice is written in it as variables — {{company}}, {{total}} — and the
// lines of the invoice go either into a table the template draws itself
// (one row with item variables, repeated for every line) or into a table
// built here in place of {{items_table}}.
package invoice

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Item is a line of an invoice.
type Item struct {
	Name     string
	Quantity float64
	Price    float64
}

// Amount is the sum of the line.
func (i Item) Amount() float64 {
	return round2(i.Quantity * i.Price)
}

// Data is everything an invoice is made of.
type Data struct {
	// The customer
	Company string
	Address string
	INN     string
	Contact string
	Phone   string
	// The contract the invoice is issued on; the dates are YYYY-MM-DD or empty
	ContractName  string
	ContractStart string
	ContractEnd   string
	// VatRate: "none" or "5"
	VatRate string
	// IssuedAt: the day the invoice is issued on
	IssuedAt time.Time
	Items    []Item
}

// Groups of variables, in the order the screen shows them.
const (
	GroupInvoice  = "invoice"
	GroupTotals   = "totals"
	GroupCustomer = "customer"
	GroupContract = "contract"
	GroupItems    = "items"
)

// Variable is something a template may contain as {{name}}.
type Variable struct {
	Name  string `json:"name"`
	Group string `json:"group"`
	// Example is what the variable turns into for the sample invoice.
	Example string `json:"example"`
}

// ItemsTable is the variable replaced by a whole table of the lines.
const ItemsTable = "items_table"

// itemVariables are filled from a line of the invoice: the table row that
// holds them is repeated for every line.
var itemVariables = []string{"item_no", "item_name", "item_qty", "item_price", "item_amount"}

// variableOrder lists every variable with its group. It is the single list
// both the substitution and the screen of the administrator work from.
var variableOrder = []struct{ name, group string }{
	{"invoice_num", GroupInvoice},
	{"date", GroupInvoice},
	{"date_ru", GroupInvoice},
	{"date_text", GroupInvoice},
	{"subtotal", GroupTotals},
	{"vat_label", GroupTotals},
	{"vat_amount", GroupTotals},
	{"total", GroupTotals},
	{"total_words", GroupTotals},
	{"items_count", GroupTotals},
	{"company", GroupCustomer},
	{"address", GroupCustomer},
	{"inn", GroupCustomer},
	{"contact", GroupCustomer},
	{"phone", GroupCustomer},
	{"contract_name", GroupContract},
	{"contract_start", GroupContract},
	{"contract_end", GroupContract},
	{ItemsTable, GroupItems},
	{"item_no", GroupItems},
	{"item_name", GroupItems},
	{"item_qty", GroupItems},
	{"item_price", GroupItems},
	{"item_amount", GroupItems},
}

// Variables returns every variable with what it gives for the sample invoice.
func Variables() []Variable {
	sample := Sample()
	values := sample.Values()
	first := itemValues(1, sample.Items[0])
	list := make([]Variable, 0, len(variableOrder))
	for _, v := range variableOrder {
		example := values[v.name]
		if item, ok := first[v.name]; ok {
			example = item
		}
		list = append(list, Variable{Name: v.name, Group: v.group, Example: example})
	}
	return list
}

func knownVariable(name string) bool {
	for _, v := range variableOrder {
		if v.name == name {
			return true
		}
	}
	return false
}

func isItemVariable(name string) bool {
	for _, v := range itemVariables {
		if v == name {
			return true
		}
	}
	return false
}

// Sample is an invented invoice: for the preview of a template and for the
// examples of the variables.
func Sample() Data {
	return Data{
		Company:       "ООО «Ромашка»",
		Address:       "620014, г. Екатеринбург, ул. Ленина, д. 1, оф. 10",
		INN:           "6671234567",
		Contact:       "Иванов Иван Иванович",
		Phone:         "+7 (343) 123-45-67",
		ContractName:  "Договор сопровождения № 12",
		ContractStart: "2026-01-01",
		ContractEnd:   "2026-12-31",
		VatRate:       "5",
		IssuedAt:      time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC),
		Items: []Item{
			{Name: "Сопровождение системы, октябрь 2026", Quantity: 1, Price: 80000},
			{Name: "Доработка отчётов", Quantity: 12.5, Price: 1600},
			{Name: "Выезд специалиста", Quantity: 2, Price: 2500.50},
		},
	}
}

func (d Data) vatFraction() float64 {
	if d.VatRate == "5" {
		return 0.05
	}
	return 0
}

// Subtotal is the sum of the lines, without VAT.
func (d Data) Subtotal() float64 {
	sum := 0.0
	for _, item := range d.Items {
		sum += item.Quantity * item.Price
	}
	return round2(sum)
}

// VAT is the tax on the subtotal.
func (d Data) VAT() float64 {
	return round2(d.Subtotal() * d.vatFraction())
}

// Total is what is to be paid.
func (d Data) Total() float64 {
	return round2(d.Subtotal() + d.VAT())
}

// Number is the number of the invoice: the day it is issued on as YYYYMM-DD.
func (d Data) Number() string {
	return d.IssuedAt.Format("200601-02")
}

// FileName is the name the invoice is downloaded under.
func (d Data) FileName() string {
	return fmt.Sprintf("Счет на оплату %s от %s.docx", d.Company, d.IssuedAt.Format("2006.01.02"))
}

// Values returns what every variable that does not depend on a line turns into.
func (d Data) Values() map[string]string {
	vatLabel := "Без НДС"
	if d.VatRate == "5" {
		vatLabel = "НДС 5%"
	}
	return map[string]string{
		"invoice_num":    d.Number(),
		"date":           d.IssuedAt.Format("2006-01-02"),
		"date_ru":        d.IssuedAt.Format("02.01.2006"),
		"date_text":      dateText(d.IssuedAt),
		"subtotal":       fmtMoney(d.Subtotal()),
		"vat_label":      vatLabel,
		"vat_amount":     fmtMoney(d.VAT()),
		"total":          fmtMoney(d.Total()),
		"total_words":    AmountInWords(d.Total()),
		"items_count":    strconv.Itoa(len(d.Items)),
		"company":        d.Company,
		"address":        d.Address,
		"inn":            d.INN,
		"contact":        d.Contact,
		"phone":          d.Phone,
		"contract_name":  d.ContractName,
		"contract_start": dayRu(d.ContractStart),
		"contract_end":   dayRu(d.ContractEnd),
	}
}

// itemValues returns what the item variables turn into for one line.
func itemValues(number int, item Item) map[string]string {
	return map[string]string{
		"item_no":     strconv.Itoa(number),
		"item_name":   item.Name,
		"item_qty":    fmtQuantity(item.Quantity),
		"item_price":  fmtMoney(item.Price),
		"item_amount": fmtMoney(item.Amount()),
	}
}

var monthsGenitive = []string{"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

// dateText writes a date the way documents do: "9 октября 2026 г.".
func dateText(t time.Time) string {
	return fmt.Sprintf("%d %s %d г.", t.Day(), monthsGenitive[t.Month()-1], t.Year())
}

// dayRu turns YYYY-MM-DD (possibly with a time part) into DD.MM.YYYY; what is
// not a date gives an empty string.
func dayRu(value string) string {
	if len(value) > 10 {
		value = value[:10]
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return ""
	}
	return t.Format("02.01.2006")
}

// fmtMoney writes an amount the Russian way: "150 000,00".
func fmtMoney(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	parts := strings.Split(s, ".")
	intPart, decPart := parts[0], parts[1]
	if n := len(intPart); n > 3 {
		var b strings.Builder
		for i, c := range intPart {
			if i > 0 && (n-i)%3 == 0 {
				b.WriteRune(' ')
			}
			b.WriteRune(c)
		}
		intPart = b.String()
	}
	if negative {
		intPart = "-" + intPart
	}
	return intPart + "," + decPart
}

// fmtQuantity writes a quantity without needless zeros: "2", "12,5".
func fmtQuantity(v float64) string {
	return strings.Replace(strconv.FormatFloat(round2(v), 'f', -1, 64), ".", ",", 1)
}

func round2(v float64) float64 {
	if v < 0 {
		return -round2(-v)
	}
	return float64(int64(v*100+0.5)) / 100
}
