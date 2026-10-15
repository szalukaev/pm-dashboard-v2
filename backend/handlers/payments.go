package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"pm-dashboard/access"
	"pm-dashboard/invoice"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

type PaymentsHandler struct {
	DB **sql.DB
	// Templates даёт шаблон, по которому формируются счета.
	Templates *InvoiceTemplates
}

// ─── Организации ───

func (h *PaymentsHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query(`SELECT o.id, o.name, o.address, o.inn, o.contact_name, o.contact_phone,
		COALESCE(NULLIF(u.display_name, ''), u.username, '')
		FROM organizations o LEFT JOIN users u ON u.id = o.user_id
		WHERE `+ownedBy("o.user_id", 1)+` ORDER BY o.name`, paymentsOwner(r))
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type Org struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		Address      *string `json:"address"`
		INN          *string `json:"inn"`
		ContactName  *string `json:"contact_name"`
		ContactPhone *string `json:"contact_phone"`
		// Owner: кто создал запись
		Owner string `json:"owner"`
	}
	var orgs []Org
	for rows.Next() {
		var o Org
		if rows.Scan(&o.ID, &o.Name, &o.Address, &o.INN, &o.ContactName, &o.ContactPhone, &o.Owner) == nil {
			orgs = append(orgs, o)
		}
	}
	if orgs == nil {
		orgs = []Org{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"organizations": orgs})
}

func (h *PaymentsHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if !allowPayment(w, r, access.PaymentOrgCreate) {
		return
	}

	userID := middleware.GetUserID(r)
	var body struct {
		Name         string  `json:"name"`
		Address      *string `json:"address"`
		INN          *string `json:"inn"`
		ContactName  *string `json:"contact_name"`
		ContactPhone *string `json:"contact_phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	var id int
	err := (*h.DB).QueryRow(`INSERT INTO organizations (user_id, name, address, inn, contact_name, contact_phone)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		userID, body.Name, body.Address, body.INN, body.ContactName, body.ContactPhone).Scan(&id)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": id, "success": true})
}

func (h *PaymentsHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if !allowPayment(w, r, access.PaymentOrgEdit) {
		return
	}

	orgID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	sets, args, idx := buildUpdateSets(body, map[string]bool{
		"name": true, "address": true, "inn": true, "contact_name": true, "contact_phone": true,
	}, 1)
	if len(sets) == 0 {
		utils.Error(w, http.StatusBadRequest, "NO_FIELDS")
		return
	}
	args = append(args, orgID, paymentsOwner(r))
	res, err := (*h.DB).Exec("UPDATE organizations SET "+utils.JoinStrings(sets, ",")+" WHERE id=$"+utils.Itoa(idx)+" AND "+ownedBy("user_id", idx+1), args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
		return
	}
	utils.Success(w)
}

func (h *PaymentsHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	if !allowPayment(w, r, access.PaymentOrgDelete) {
		return
	}
	orgID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Договоры не удаляются, а отвязываются от организации: так задан
	// внешний ключ (ON DELETE SET NULL)
	res, err := (*h.DB).Exec("DELETE FROM organizations WHERE id = $1 AND "+ownedBy("user_id", 2), orgID, paymentsOwner(r))
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
		return
	}
	utils.Success(w)
}

// ─── Договоры ───

// paymentContract — договор вместе с тем, что для него посчитано.
type paymentContract struct {
	ID             int     `json:"id"`
	OrganizationID *int    `json:"organization_id"`
	ContractType   string  `json:"contract_type"`
	Name           string  `json:"name"`
	CompanyName    *string `json:"company_name"`
	CompanyAddress *string `json:"company_address"`
	Amount         float64 `json:"amount"`
	VatRate        string  `json:"vat_rate"`
	ContactName    *string `json:"contact_name"`
	ContactPhone   *string `json:"contact_phone"`
	StartDate      *string `json:"start_date"`
	EndDate        *string `json:"end_date"`
	OrgName        string  `json:"org_name"`
	// Owner: кто создал договор
	Owner string `json:"owner"`
	// TotalAmount: сумма договора с НДС.
	TotalAmount float64 `json:"total_amount"`
	// InvoicedAmount: сумма счетов с НДС.
	InvoicedAmount float64 `json:"invoiced_amount"`
	TotalPaid      float64 `json:"total_paid"`
	// DebtAmount: выставлено и ещё не оплачено.
	DebtAmount  float64 `json:"debt_amount"`
	IsClosed    bool    `json:"is_closed"`
	IsOverdue   bool    `json:"is_overdue"`
	IsFullyPaid bool    `json:"is_fully_paid"`

	money contractMoney
}

// contractFilter — чем сужается список договоров.
type contractFilter struct {
	Type       string // "service" | "onetime" | "" — оба вида
	Search     string // по названию договора или организации
	ShowClosed bool
}

func contractFilterOf(r *http.Request) contractFilter {
	q := r.URL.Query()
	return contractFilter{
		Type:       q.Get("type"),
		Search:     strings.TrimSpace(q.Get("search")),
		ShowClosed: q.Get("show_closed") == "true",
	}
}

// loadContracts возвращает видимые пользователю договоры с их посчитанным
// состоянием; owner — значение paymentsOwner. Фильтр nil даёт все договоры.
func (h *PaymentsHandler) loadContracts(owner interface{}, filter *contractFilter) ([]paymentContract, error) {
	where := []string{ownedBy("c.user_id", 1)}
	args := []interface{}{owner}
	if filter != nil && filter.Type != "" {
		args = append(args, filter.Type)
		where = append(where, "c.contract_type = $"+utils.Itoa(len(args)))
	}
	if filter != nil && filter.Search != "" {
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		n := utils.Itoa(len(args))
		where = append(where, "(LOWER(c.name) LIKE $"+n+" OR LOWER(COALESCE(c.company_name, '')) LIKE $"+n+
			" OR LOWER(COALESCE(o.name, '')) LIKE $"+n+")")
	}

	// #nosec G701 -- условия фильтра и ownedBy — константы из кода, LIMIT/OFFSET — числа, значения через $n
	rows, err := (*h.DB).Query(`SELECT c.id, c.organization_id, c.contract_type, c.name, c.company_name, c.company_address,
		c.amount, c.vat_rate, c.contact_name, c.contact_phone, c.start_date, c.end_date,
		COALESCE(o.name, '') AS org_name,
		COALESCE(NULLIF(u.display_name, ''), u.username, '') AS owner,
		COALESCE((SELECT SUM(i.amount * CASE WHEN i.vat_rate = '5' THEN 1.05 ELSE 1 END)
			FROM invoices i WHERE i.contract_id = c.id), 0) AS invoiced_amount,
		COALESCE((SELECT SUM(p.amount) FROM contract_payments p
			JOIN invoices i ON p.invoice_id = i.id WHERE i.contract_id = c.id), 0) AS total_paid
		FROM contracts c LEFT JOIN organizations o ON c.organization_id = o.id
		LEFT JOIN users u ON u.id = c.user_id
		WHERE `+utils.JoinStrings(where, " AND ")+` ORDER BY c.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contracts := []paymentContract{}
	today := time.Now().Format("2006-01-02")
	for rows.Next() {
		var c paymentContract
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.ContractType, &c.Name, &c.CompanyName, &c.CompanyAddress,
			&c.Amount, &c.VatRate, &c.ContactName, &c.ContactPhone, &c.StartDate, &c.EndDate,
			&c.OrgName, &c.Owner, &c.InvoicedAmount, &c.TotalPaid); err != nil {
			return nil, err
		}
		c.InvoicedAmount = utils.Round2(c.InvoicedAmount)
		c.money = contractMoney{Type: c.ContractType, Amount: c.Amount, VatRate: c.VatRate,
			Invoiced: c.InvoicedAmount, Paid: c.TotalPaid, EndDate: c.EndDate}
		c.TotalAmount = c.money.Total()
		c.DebtAmount = c.money.Debt()
		c.IsFullyPaid = c.money.FullyPaid()
		c.IsClosed = c.money.Closed(today)
		c.IsOverdue = c.money.Overdue(today)

		if filter != nil && !filter.ShowClosed && c.IsClosed {
			continue
		}
		contracts = append(contracts, c)
	}
	return contracts, rows.Err()
}

func (h *PaymentsHandler) ListContracts(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	filter := contractFilterOf(r)
	contracts, err := h.loadContracts(paymentsOwner(r), &filter)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"contracts": contracts})
}

func (h *PaymentsHandler) CreateContract(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if !allowPayment(w, r, access.PaymentContractCreate) {
		return
	}

	userID := middleware.GetUserID(r)
	var body struct {
		ContractType   string   `json:"contract_type"`
		Name           string   `json:"name"`
		OrganizationID *int     `json:"organization_id"`
		CompanyName    *string  `json:"company_name"`
		CompanyAddress *string  `json:"company_address"`
		Amount         *float64 `json:"amount"`
		VatRate        *string  `json:"vat_rate"`
		ContactName    *string  `json:"contact_name"`
		ContactPhone   *string  `json:"contact_phone"`
		StartDate      *string  `json:"start_date"`
		EndDate        *string  `json:"end_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if body.Name == "" || body.ContractType == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	// Данные берутся из организации, если она указана (только видимая)
	if body.OrganizationID != nil {
		var orgName, orgAddr, orgContact, orgPhone sql.NullString
		err := (*h.DB).QueryRow("SELECT name, address, contact_name, contact_phone FROM organizations WHERE id=$1 AND "+ownedBy("user_id", 2), *body.OrganizationID, paymentsOwner(r)).Scan(&orgName, &orgAddr, &orgContact, &orgPhone)
		if err != nil {
			// Невидимая или несуществующая организация — отказ, а не привязка к чужому.
			utils.Error(w, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
			return
		}
		if body.CompanyName == nil && orgName.Valid {
			body.CompanyName = &orgName.String
		}
		if body.CompanyAddress == nil && orgAddr.Valid {
			body.CompanyAddress = &orgAddr.String
		}
		if body.ContactName == nil && orgContact.Valid {
			body.ContactName = &orgContact.String
		}
		if body.ContactPhone == nil && orgPhone.Valid {
			body.ContactPhone = &orgPhone.String
		}
	}
	// Дата, не заполненная в форме, — «не задана», а не пустая строка
	body.StartDate, body.EndDate = emptyToNil(body.StartDate), emptyToNil(body.EndDate)
	vatRate := "none"
	if body.VatRate != nil {
		vatRate = *body.VatRate
	}
	amount := 0.0
	if body.Amount != nil {
		amount = *body.Amount
	}

	var id int
	err := (*h.DB).QueryRow(`INSERT INTO contracts (user_id, organization_id, contract_type, name, company_name,
		company_address, amount, vat_rate, contact_name, contact_phone, start_date, end_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		userID, body.OrganizationID, body.ContractType, body.Name, body.CompanyName,
		body.CompanyAddress, amount, vatRate, body.ContactName, body.ContactPhone,
		body.StartDate, body.EndDate).Scan(&id)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": id, "success": true})
}

func (h *PaymentsHandler) UpdateContract(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if !allowPayment(w, r, access.PaymentContractEdit) {
		return
	}

	contractID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	// Дата, не заполненная в форме, — «не задана», а не пустая строка
	for _, field := range []string{"start_date", "end_date"} {
		if text, isText := body[field].(string); isText && strings.TrimSpace(text) == "" {
			body[field] = nil
		}
	}
	sets, args, idx := buildUpdateSets(body, map[string]bool{
		"name": true, "contract_type": true, "organization_id": true, "company_name": true,
		"company_address": true, "amount": true, "vat_rate": true, "contact_name": true,
		"contact_phone": true, "start_date": true, "end_date": true,
	}, 1)
	if len(sets) == 0 {
		utils.Error(w, http.StatusBadRequest, "NO_FIELDS")
		return
	}
	args = append(args, contractID, paymentsOwner(r))
	res, err := (*h.DB).Exec("UPDATE contracts SET "+utils.JoinStrings(sets, ",")+" WHERE id=$"+utils.Itoa(idx)+" AND "+ownedBy("user_id", idx+1), args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "CONTRACT_NOT_FOUND")
		return
	}
	utils.Success(w)
}

func (h *PaymentsHandler) DeleteContract(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	if !allowPayment(w, r, access.PaymentContractDelete) {
		return
	}
	contractID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	res, err := (*h.DB).Exec("DELETE FROM contracts WHERE id=$1 AND "+ownedBy("user_id", 2), contractID, paymentsOwner(r))
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "CONTRACT_NOT_FOUND")
		return
	}
	utils.Success(w)
}

// ─── Счета ───

func (h *PaymentsHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	contractID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Счета видны только через видимый пользователю договор; невидимый или
	// несуществующий договор — 404, как в остальных обработчиках оплат
	owner := paymentsOwner(r)
	var visible bool
	if err := (*h.DB).QueryRow(`SELECT EXISTS(SELECT 1 FROM contracts c WHERE c.id = $1 AND `+ownedBy("c.user_id", 2)+`)`,
		contractID, owner).Scan(&visible); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	if !visible {
		utils.Error(w, http.StatusNotFound, "CONTRACT_NOT_FOUND")
		return
	}
	limit, offset := pageOf(r)
	total := 0
	if err := (*h.DB).QueryRow(`SELECT COUNT(*) FROM invoices i JOIN contracts c ON i.contract_id = c.id
		WHERE i.contract_id=$1 AND `+ownedBy("c.user_id", 2), contractID, owner).Scan(&total); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	// #nosec G701 -- условия фильтра и ownedBy — константы из кода, LIMIT/OFFSET — числа, значения через $n
	rows, err := (*h.DB).Query(`SELECT i.id, i.amount, i.vat_rate, i.issued_at, i.paid_amount, i.status,
			(SELECT MAX(p.paid_at)::text FROM contract_payments p WHERE p.invoice_id = i.id)
		FROM invoices i JOIN contracts c ON i.contract_id = c.id
		WHERE i.contract_id=$1 AND `+ownedBy("c.user_id", 2)+` ORDER BY i.issued_at DESC, i.id DESC`+pageClause(limit, offset), contractID, owner)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type Invoice struct {
		ID         int     `json:"id"`
		Amount     float64 `json:"amount"`
		VatRate    string  `json:"vat_rate"`
		IssuedAt   string  `json:"issued_at"`
		PaidAmount float64 `json:"paid_amount"`
		Status     string  `json:"status"`
		Total      float64 `json:"total"`
		Remainder  float64 `json:"remainder"`
		// Number: дата выставления в виде YYYYMM-DD
		Number string `json:"number"`
		// PaidAt: дата последней оплаты; nil — ничего не оплачено
		PaidAt *string `json:"paid_at"`
	}
	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		if rows.Scan(&inv.ID, &inv.Amount, &inv.VatRate, &inv.IssuedAt, &inv.PaidAmount, &inv.Status, &inv.PaidAt) != nil {
			continue
		}
		inv.Total = utils.Round2(inv.Amount * vatMultiplier(inv.VatRate))
		inv.Remainder = utils.Round2(inv.Total - inv.PaidAmount)
		if inv.Remainder < 0 {
			inv.Remainder = 0
		}
		inv.Number = invoiceNumber(inv.IssuedAt)
		invoices = append(invoices, inv)
	}
	if invoices == nil {
		invoices = []Invoice{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"invoices": invoices, "total": total})
}

// maxInvoiceItemName ограничивает название позиции счёта: хватает на ссылку
// на договор с приложением, и при этом название помещается при печати.
const maxInvoiceItemName = 2000

func (h *PaymentsHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	if !allowPayment(w, r, access.PaymentInvoiceCreate) {
		return
	}
	contractID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var contract struct {
		Amount  float64
		VatRate string
		Name    string
	}
	err = (*h.DB).QueryRow("SELECT amount, vat_rate, name FROM contracts WHERE id=$1 AND "+ownedBy("user_id", 2), contractID, paymentsOwner(r)).Scan(&contract.Amount, &contract.VatRate, &contract.Name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "CONTRACT_NOT_FOUND")
		return
	}

	var body struct {
		Items []struct {
			Name     string
			Quantity float64
			Price    float64
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	if len(body.Items) == 0 {
		body.Items = []struct {
			Name     string
			Quantity float64
			Price    float64
		}{{Name: contract.Name, Quantity: 1, Price: contract.Amount}}
	}

	totalAmount := 0.0
	for i, item := range body.Items {
		name := strings.TrimSpace(item.Name)
		switch {
		case name == "":
			utils.Error(w, http.StatusBadRequest, "ITEM_NAME_REQUIRED")
			return
		case utf8.RuneCountInString(name) > maxInvoiceItemName:
			utils.Error(w, http.StatusBadRequest, "ITEM_NAME_TOO_LONG")
			return
		case item.Quantity < 0 || item.Price < 0:
			utils.Error(w, http.StatusBadRequest, "INVALID_AMOUNT")
			return
		}
		body.Items[i].Name = name
		totalAmount += item.Quantity * item.Price
	}
	totalAmount = utils.Round2(totalAmount)

	today := time.Now().Format("2006-01-02")

	// Счёт и его позиции записываются одной транзакцией — иначе при сбое
	// посередине остался бы счёт без позиций.
	tx, err := (*h.DB).Begin()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	var invoiceID int
	err = tx.QueryRow(`INSERT INTO invoices (contract_id, amount, vat_rate, issued_at, paid_amount, status)
		VALUES ($1,$2,$3,$4,0,'unpaid') RETURNING id`, contractID, totalAmount, contract.VatRate, today).Scan(&invoiceID)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("Could not save the invoice", "contract", contractID, "error", err)
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	for _, item := range body.Items {
		if _, err := tx.Exec("INSERT INTO invoice_items (invoice_id, name, quantity, price) VALUES ($1,$2,$3,$4)",
			invoiceID, item.Name, item.Quantity, item.Price); err != nil {
			_ = tx.Rollback()
			slog.Error("Could not save a line of the invoice", "contract", contractID, "error", err)
			utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": invoiceID, "success": true})
}

// GetInvoiceItems возвращает позиции счёта: по ним можно выставить новый
// счёт.
func (h *PaymentsHandler) GetInvoiceItems(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	invoiceID, err := strconv.Atoi(mux.Vars(r)["invoiceId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Только счёт по видимому пользователю договору
	var amount float64
	var contractName string
	err = (*h.DB).QueryRow(`SELECT i.amount, c.name FROM invoices i JOIN contracts c ON i.contract_id = c.id
		WHERE i.id=$1 AND `+ownedBy("c.user_id", 2), invoiceID, paymentsOwner(r)).Scan(&amount, &contractName)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND")
		return
	}

	type item struct {
		Name     string  `json:"name"`
		Quantity float64 `json:"quantity"`
		Price    float64 `json:"price"`
	}
	items := []item{}
	rows, err := (*h.DB).Query("SELECT name, quantity, price FROM invoice_items WHERE invoice_id=$1 ORDER BY id", invoiceID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var it item
		if rows.Scan(&it.Name, &it.Quantity, &it.Price) == nil {
			items = append(items, it)
		}
	}
	// У старого счёта позиции могут быть не сохранены: он был одной строкой договора
	if len(items) == 0 {
		items = append(items, item{Name: contractName, Quantity: 1, Price: amount})
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (h *PaymentsHandler) DeleteInvoice(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	if !allowPayment(w, r, access.PaymentInvoiceDelete) {
		return
	}
	invoiceID, err := strconv.Atoi(mux.Vars(r)["invoiceId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	// Удалить можно только счёт по видимому договору.
	res, err := (*h.DB).Exec(`DELETE FROM invoices WHERE id=$1 AND contract_id IN
		(SELECT id FROM contracts WHERE `+ownedBy("user_id", 2)+`)`, invoiceID, paymentsOwner(r))
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND")
		return
	}
	utils.Success(w)
}

func (h *PaymentsHandler) PayInvoice(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	if !allowPayment(w, r, access.PaymentInvoicePay) {
		return
	}
	invoiceID, err := strconv.Atoi(mux.Vars(r)["invoiceId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var body struct {
		Amount *float64 `json:"amount"`
		PaidAt *string  `json:"paid_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Остаток читается и меняется в одной транзакции под блокировкой строки
	// счёта: одновременные оплаты идут по очереди, и каждая видит сумму,
	// уже записанную предыдущей.
	tx, err := (*h.DB).Begin()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "TX_FAILED")
		return
	}
	defer tx.Rollback()

	// Оплатить можно только счёт по видимому договору.
	var inv struct {
		Amount     float64
		VatRate    string
		PaidAmount float64
	}
	err = tx.QueryRow(`SELECT i.amount, i.vat_rate, i.paid_amount
		FROM invoices i JOIN contracts c ON i.contract_id = c.id
		WHERE i.id=$1 AND `+ownedBy("c.user_id", 2)+` FOR UPDATE OF i`, invoiceID, paymentsOwner(r)).Scan(&inv.Amount, &inv.VatRate, &inv.PaidAmount)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND")
		return
	}

	totalObligation := utils.Round2(inv.Amount * vatMultiplier(inv.VatRate))
	remainder := totalObligation - inv.PaidAmount

	// Сумма больше остатка урезается до остатка; ответ об этом сообщает, и
	// страница говорит пользователю, сколько принято.
	payAmount, clamped, ok := appliedPayment(body.Amount, remainder)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_AMOUNT")
		return
	}

	payDate := time.Now().Format("2006-01-02")
	if body.PaidAt != nil {
		payDate = *body.PaidAt
	}

	newPaid := utils.Round2(inv.PaidAmount + payAmount)
	if newPaid > totalObligation {
		newPaid = totalObligation
	}
	status := "partial"
	if newPaid >= totalObligation {
		status = "paid"
	}

	// Счёт и строка оплаты меняются вместе: при сбое одного запроса
	// состояние не расходится.
	if _, err := tx.Exec("UPDATE invoices SET paid_amount=$1, status=$2 WHERE id=$3", newPaid, status, invoiceID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if _, err := tx.Exec("INSERT INTO contract_payments (invoice_id, amount, paid_at) VALUES ($1,$2,$3)", invoiceID, payAmount, payDate); err != nil {
		utils.Error(w, http.StatusInternalServerError, "INSERT_FAILED")
		return
	}
	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "COMMIT_FAILED")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "paid_amount": newPaid, "status": status,
		"applied": payAmount, "clamped": clamped,
	})
}

func (h *PaymentsHandler) DownloadInvoice(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	if !allowPayment(w, r, access.PaymentDownload) {
		return
	}
	invoiceID, err := strconv.Atoi(mux.Vars(r)["invoiceId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	// Счёт с его договором и организацией (только видимые)
	var issuedAt string
	var amount float64
	data := invoice.Data{}
	err = (*h.DB).QueryRow(`SELECT i.amount, i.vat_rate, i.issued_at::text,
			c.name, COALESCE(c.company_name, o.name, ''), COALESCE(c.company_address, o.address, ''),
			COALESCE(c.contact_phone, ''), COALESCE(c.contact_name, ''), COALESCE(o.inn, ''),
			COALESCE(c.start_date::text, ''), COALESCE(c.end_date::text, '')
		FROM invoices i JOIN contracts c ON i.contract_id = c.id
		LEFT JOIN organizations o ON o.id = c.organization_id
		WHERE i.id=$1 AND `+ownedBy("c.user_id", 2), invoiceID, paymentsOwner(r)).Scan(
		&amount, &data.VatRate, &issuedAt,
		&data.ContractName, &data.Company, &data.Address,
		&data.Phone, &data.Contact, &data.INN,
		&data.ContractStart, &data.ContractEnd)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "INVOICE_NOT_FOUND")
		return
	}
	// Номер и дата документа берутся от дня выставления счёта: повторное
	// скачивание даёт тот же документ
	data.IssuedAt, err = time.ParseInLocation("2006-01-02", firstN(issuedAt, 10), time.Local)
	if err != nil {
		data.IssuedAt = time.Now()
	}

	rows, err := (*h.DB).Query("SELECT name, quantity, price FROM invoice_items WHERE invoice_id=$1 ORDER BY id", invoiceID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item invoice.Item
		if rows.Scan(&item.Name, &item.Quantity, &item.Price) == nil {
			data.Items = append(data.Items, item)
		}
	}
	if len(data.Items) == 0 {
		data.Items = []invoice.Item{{Name: data.ContractName, Quantity: 1, Price: amount}}
	}

	template, _, err := h.Templates.Current()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "TEMPLATE_NOT_FOUND")
		return
	}
	document, err := invoice.Render(template, data)
	if err != nil {
		slog.Error("Could not make the invoice document", "invoice", invoiceID, "error", err)
		utils.Error(w, http.StatusInternalServerError, "DOCX_GENERATION_FAILED")
		return
	}
	serveDocx(w, data.FileName(), document)
}

// ─── Статистика ───

// GetStats возвращает цифры карточек над списком. Они всегда считаются по
// всем видимым пользователю договорам, что бы ни показывали фильтры.
func (h *PaymentsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	contracts, err := h.loadContracts(paymentsOwner(r), nil)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	money := make([]contractMoney, len(contracts))
	for i, c := range contracts {
		money[i] = c.money
	}
	totals := sumContracts(money, time.Now().Format("2006-01-02"))

	stats := []map[string]interface{}{
		{"label": "contracts", "value": totals.Contracts},
		{"label": "total_amount", "value": totals.TotalAmount},
		{"label": "invoiced", "value": totals.Invoiced},
		{"label": "debt", "value": totals.Debt},
		{"label": "paid", "value": totals.Paid},
		{"label": "closed_amount", "value": totals.ClosedAmount},
	}
	// Карточка задолженности красная, только когда долг есть
	if totals.Debt > 0 {
		stats[3]["variant"] = "danger"
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"stats": stats})
}

// ─── Экспорт в CSV ───

// ExportCSV выгружает договоры, показанные в списке, — с теми же фильтрами.
func (h *PaymentsHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if !allowPayment(w, r, access.PaymentDownload) {
		return
	}
	filter := contractFilterOf(r)
	contracts, err := h.loadContracts(paymentsOwner(r), &filter)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	sort.SliceStable(contracts, func(i, j int) bool {
		return strings.ToLower(contracts[i].Name) < strings.ToLower(contracts[j].Name)
	})

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=contracts.csv")
	// BOM — для Excel
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';'
	for _, row := range contractsCSV(contracts, time.Now().Format("2006-01-02")) {
		_ = writer.Write(row)
	}
	writer.Flush()
}

// contractsCSV собирает строки выгрузки, первой — шапку. Суммы — с НДС и с
// десятичной запятой, как их ждут здешние электронные таблицы.
func contractsCSV(contracts []paymentContract, today string) [][]string {
	text := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	money := func(v float64) string {
		return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1)
	}
	rows := [][]string{{"Тип", "Название", "Организация", "Адрес", "Сумма", "НДС", "Выставлено", "Долг",
		"Контакт", "Телефон", "Дата начала", "Срок", "Статус"}}
	for _, c := range contracts {
		typeName := "Сопровождение"
		if c.ContractType == "onetime" {
			typeName = "Разовый"
		}
		vatLabel := "Без НДС"
		if c.VatRate == "5" {
			vatLabel = "НДС 5%"
		}
		organization := c.OrgName
		if organization == "" {
			organization = text(c.CompanyName)
		}
		rows = append(rows, []string{typeName, c.Name, organization, text(c.CompanyAddress),
			money(c.TotalAmount), vatLabel, money(c.InvoicedAmount), money(c.DebtAmount),
			text(c.ContactName), text(c.ContactPhone),
			firstN(text(c.StartDate), 10), firstN(text(c.EndDate), 10), c.money.ExportStatus(today)})
	}
	return rows
}

// ─── Вспомогательное ───

// paymentsOwner возвращает параметр для ownedBy: автора, если пользователю
// видны только свои записи, иначе NULL — видны все.
func paymentsOwner(r *http.Request) interface{} {
	if id := middleware.GetScope(r).PaymentsOwner(); id != nil {
		return *id
	}
	return nil
}

// ownedBy — условие видимости записи «Оплат»: column — столбец автора, n —
// номер параметра со значением paymentsOwner.
func ownedBy(column string, n int) string {
	p := "$" + utils.Itoa(n)
	return "(" + p + "::int IS NULL OR " + column + " = " + p + ")"
}

// allowPayment проверяет действие с оплатами (одна из констант
// access.Payment*) и отвечает 403, если оно запрещено.
func allowPayment(w http.ResponseWriter, r *http.Request, action string) bool {
	if middleware.GetScope(r).CanPayment(action) {
		return true
	}
	slog.Warn("Payment action outside of user rights", "user", middleware.GetUserID(r), "action", action)
	utils.Error(w, http.StatusForbidden, "PAYMENT_ACTION_FORBIDDEN")
	return false
}

func buildUpdateSets(body map[string]interface{}, allowed map[string]bool, startIdx int) ([]string, []interface{}, int) {
	sets := []string{}
	args := []interface{}{}
	idx := startIdx
	for k, v := range body {
		if allowed[k] {
			sets = append(sets, k+"=$"+utils.Itoa(idx))
			args = append(args, v)
			idx++
		}
	}
	return sets, args, idx
}
