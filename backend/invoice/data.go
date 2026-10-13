// Package invoice заполняет шаблон .docx данными счёта.
//
// Шаблон — обычный документ Word. То, что меняется от счёта к счёту,
// записано в нём переменными — {{company}}, {{total}}, — а позиции счёта
// попадают либо в таблицу, которую рисует сам шаблон (одна строка с
// переменными позиции, повторяемая для каждой позиции), либо в таблицу,
// которая строится здесь на месте {{items_table}}.
package invoice

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Item — позиция счёта.
type Item struct {
	Name     string
	Quantity float64
	Price    float64
}

// Amount — сумма позиции.
func (i Item) Amount() float64 {
	return round2(i.Quantity * i.Price)
}

// Data — всё, из чего состоит счёт.
type Data struct {
	// Заказчик
	Company string
	Address string
	INN     string
	Contact string
	Phone   string
	// Договор, по которому выставлен счёт; даты — YYYY-MM-DD или пусто
	ContractName  string
	ContractStart string
	ContractEnd   string
	// VatRate: "none" или "5"
	VatRate string
	// IssuedAt: день выставления счёта
	IssuedAt time.Time
	Items    []Item
}

// Группы переменных в том порядке, в каком их показывает экран.
const (
	GroupInvoice  = "invoice"
	GroupTotals   = "totals"
	GroupCustomer = "customer"
	GroupContract = "contract"
	GroupItems    = "items"
)

// Variable — то, что шаблон может содержать в виде {{name}}.
type Variable struct {
	Name  string `json:"name"`
	Group string `json:"group"`
	// Example — во что переменная превращается в счёте-образце.
	Example string `json:"example"`
}

// ItemsTable — переменная, на месте которой строится вся таблица позиций.
const ItemsTable = "items_table"

// itemVariables заполняются из позиции счёта: строка таблицы, в которой
// они стоят, повторяется для каждой позиции.
var itemVariables = []string{"item_no", "item_name", "item_qty", "item_price", "item_amount"}

// variableOrder перечисляет все переменные с их группами. Это единый
// список, по которому работают и подстановка, и экран администратора.
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

// Variables возвращает все переменные с их значениями для счёта-образца.
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

// Sample — вымышленный счёт: для предпросмотра шаблона и для примеров
// значений переменных.
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

// Subtotal — сумма позиций без НДС.
func (d Data) Subtotal() float64 {
	sum := 0.0
	for _, item := range d.Items {
		sum += item.Quantity * item.Price
	}
	return round2(sum)
}

// VAT — НДС с суммы позиций.
func (d Data) VAT() float64 {
	return round2(d.Subtotal() * d.vatFraction())
}

// Total — сумма к оплате.
func (d Data) Total() float64 {
	return round2(d.Subtotal() + d.VAT())
}

// Number — номер счёта: день выставления в виде YYYYMM-DD.
func (d Data) Number() string {
	return d.IssuedAt.Format("200601-02")
}

// FileName — имя, под которым счёт скачивается.
func (d Data) FileName() string {
	return fmt.Sprintf("Счет на оплату %s от %s.docx", d.Company, d.IssuedAt.Format("2006.01.02"))
}

// Values возвращает значения всех переменных, не зависящих от позиции.
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

// itemValues возвращает значения переменных позиции для одной позиции.
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

// dayRu переводит YYYY-MM-DD (возможно, со временем) в DD.MM.YYYY; то, что
// датой не является, даёт пустую строку.
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

// fmtMoney записывает сумму так, как принято в России: "150 000,00".
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

// fmtQuantity записывает количество без лишних нулей: "2", "12,5".
func fmtQuantity(v float64) string {
	return strings.Replace(strconv.FormatFloat(round2(v), 'f', -1, 64), ".", ",", 1)
}

func round2(v float64) float64 {
	if v < 0 {
		return -round2(-v)
	}
	return float64(int64(v*100+0.5)) / 100
}
